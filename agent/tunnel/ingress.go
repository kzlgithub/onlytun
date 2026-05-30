package tunnel

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/onlytun/agent/frame"
	"github.com/onlytun/agent/session"
)

const (
	stopWaitTimeout       = 30 * time.Second
	copyBufferSize        = 32 * 1024
	udpIdleTimeout        = 30 * time.Second
	udpCleanupInterval    = 5 * time.Second
	udpSessionLimit       = 1024
	udpSessionSendQueue   = 256
)

var (
	errIngressStarted    = errors.New("tunnel: ingress already started")
	errUnsupportedProto  = errors.New("tunnel: unsupported protocol")
	errIngressNotStarted = errors.New("tunnel: ingress not started")
)

// IngressConfig 入口机转发规则配置。
type IngressConfig struct {
	ListenAddr string
	Protocol   string
	EgressAddr string
	TargetAddr string
	PSK        []byte
	RuleID     string
}

// Stats 流量统计。
type Stats struct {
	ActiveConns     int64
	BytesUp         int64
	BytesDown       int64
	UDPSessions     int64
	UDPDropped      int64
	UDPDecodeErrors int64
	UDPReplayErrors int64
}

// IngressTunnel 入口机隧道。
type IngressTunnel struct {
	cfg    IngressConfig
	ctx    context.Context
	cancel context.CancelFunc

	startMu sync.Mutex
	wg      sync.WaitGroup

	tcpListener net.Listener
	udpConn     *net.UDPConn

	activeConns atomic.Int64
	bytesUp     atomic.Int64
	bytesDown   atomic.Int64
	udpDropped  atomic.Int64
	udpDecodeErrors atomic.Int64
	udpReplayErrors atomic.Int64

	closerMu sync.Mutex
	closers  map[io.Closer]struct{}

	udpMu       sync.Mutex
	udpSessions map[string]*udpClientSession
}

type udpClientSession struct {
	clientAddr *net.UDPAddr
	egressConn net.Conn
	framer     *frame.TCPFramer
	udpEncoder *frame.UDPFramer
	udpDecoder *frame.UDPFramer

	sendQueue chan []byte
	done      chan struct{}
	lastSeen  atomic.Int64
	closeOnce sync.Once
}

// NewIngressTunnel 创建入口机隧道。
func NewIngressTunnel(cfg IngressConfig) *IngressTunnel {
	ctx, cancel := context.WithCancel(context.Background())
	return &IngressTunnel{
		cfg:         cfg,
		ctx:         ctx,
		cancel:      cancel,
		closers:     make(map[io.Closer]struct{}),
		udpSessions: make(map[string]*udpClientSession),
	}
}

// Start 开始监听，非阻塞，在后台 goroutine 中运行。
func (t *IngressTunnel) Start() error {
	t.startMu.Lock()
	defer t.startMu.Unlock()

	if t.tcpListener != nil || t.udpConn != nil {
		return errIngressStarted
	}

	proto := strings.ToLower(t.cfg.Protocol)
	switch proto {
	case "tcp":
		ln, err := net.Listen("tcp", t.cfg.ListenAddr)
		if err != nil {
			return err
		}
		t.tcpListener = ln
		t.wg.Add(1)
		go t.runTCPAcceptLoop()
	case "udp":
		udpConn, err := t.listenUDP()
		if err != nil {
			return err
		}
		t.udpConn = udpConn
		t.wg.Add(2)
		go t.runUDPAcceptLoop()
		go t.runUDPCleanupLoop()
	case "both":
		ln, err := net.Listen("tcp", t.cfg.ListenAddr)
		if err != nil {
			return err
		}
		udpConn, err := t.listenUDP()
		if err != nil {
			ln.Close()
			return err
		}
		t.tcpListener = ln
		t.udpConn = udpConn
		t.wg.Add(3)
		go t.runTCPAcceptLoop()
		go t.runUDPAcceptLoop()
		go t.runUDPCleanupLoop()
	default:
		return errUnsupportedProto
	}

	return nil
}

// Stop 停止监听，等待所有活跃连接自然结束（最多等待 30 秒）。
func (t *IngressTunnel) Stop() {
	t.cancel()

	t.startMu.Lock()
	if t.tcpListener != nil {
		_ = t.tcpListener.Close()
	}
	if t.udpConn != nil {
		_ = t.udpConn.Close()
	}
	t.startMu.Unlock()

	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(stopWaitTimeout):
		t.closeAllActive()
		<-done
	}
}

// GetStats 获取当前统计数据。
func (t *IngressTunnel) GetStats() Stats {
	t.udpMu.Lock()
	udpSessions := len(t.udpSessions)
	t.udpMu.Unlock()

	return Stats{
		ActiveConns:     t.activeConns.Load(),
		BytesUp:         t.bytesUp.Load(),
		BytesDown:       t.bytesDown.Load(),
		UDPSessions:     int64(udpSessions),
		UDPDropped:      t.udpDropped.Load(),
		UDPDecodeErrors: t.udpDecodeErrors.Load(),
		UDPReplayErrors: t.udpReplayErrors.Load(),
	}
}

func (t *IngressTunnel) runTCPAcceptLoop() {
	defer t.wg.Done()

	for {
		conn, err := t.tcpListener.Accept()
		if err != nil {
			if t.ctx.Err() != nil {
				return
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return
		}

		configureTCPConn(conn)
		t.wg.Add(1)
		go t.handleTCPClient(conn)
	}
}

func (t *IngressTunnel) runUDPAcceptLoop() {
	defer t.wg.Done()

	buf := make([]byte, udpDatagramReadSize())
	for {
		n, clientAddr, err := t.udpConn.ReadFromUDP(buf)
		if err != nil {
			if t.ctx.Err() != nil {
				return
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			return
		}

		payload := append([]byte(nil), buf[:n]...)
		sess, err := t.getOrCreateUDPSession(clientAddr)
		if err != nil {
			t.udpDropped.Add(1)
			continue
		}

		if !sess.enqueue(payload) {
			t.udpDropped.Add(1)
			continue
		}

		t.bytesUp.Add(int64(len(payload)))
	}
}

func (t *IngressTunnel) handleTCPClient(clientConn net.Conn) {
	defer t.wg.Done()

	egressConn, framer, _, err := t.openEgressTunnel()
	if err != nil {
		clientConn.Close()
		return
	}

	t.activeConns.Add(1)
	t.trackCloser(clientConn)
	t.trackCloser(egressConn)
	defer func() {
		t.untrackCloser(clientConn)
		t.untrackCloser(egressConn)
		t.activeConns.Add(-1)
	}()

	if err := framer.WriteFrame(egressConn, encodeTargetAddr(1, t.cfg.TargetAddr)); err != nil {
		clientConn.Close()
		egressConn.Close()
		return
	}

	t.pipeTCP(clientConn, egressConn, framer)
}

func (t *IngressTunnel) pipeTCP(clientConn net.Conn, egressConn net.Conn, framer *frame.TCPFramer) {
	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			_ = clientConn.Close()
			_ = egressConn.Close()
		})
	}

	errCh := make(chan error, 2)

	go func() {
		buf := acquireCopyBuffer()
		defer releaseCopyBuffer(buf)
		for {
			n, err := clientConn.Read(buf)
			if n > 0 {
				if writeErr := framer.WriteFrame(egressConn, buf[:n]); writeErr != nil {
					errCh <- writeErr
					return
				}
				t.bytesUp.Add(int64(n))
			}
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		for {
			payload, err := framer.ReadFrame(egressConn)
			if len(payload) > 0 {
				if _, writeErr := clientConn.Write(payload); writeErr != nil {
					errCh <- writeErr
					return
				}
				t.bytesDown.Add(int64(len(payload)))
			}
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	select {
	case <-t.ctx.Done():
	case <-errCh:
	}
	closeBoth()
}

func (t *IngressTunnel) getOrCreateUDPSession(clientAddr *net.UDPAddr) (*udpClientSession, error) {
	key := clientAddr.String()
	now := time.Now()

	t.udpMu.Lock()
	if sess, ok := t.udpSessions[key]; ok {
		sess.touch(now)
		t.udpMu.Unlock()
		return sess, nil
	}
	evictKey := ""
	if len(t.udpSessions) >= udpSessionLimit {
		evictKey = t.oldestUDPSessionKeyLocked()
	}
	t.udpMu.Unlock()

	if evictKey != "" {
		t.closeUDPSession(evictKey)
	}

	egressConn, framer, result, err := t.openEgressTunnel()
	if err != nil {
		return nil, err
	}

	udpEncoder, err := frame.NewUDPFramer(t.cfg.PSK, result.C2SEncKey, result.C2SAuthKey, result.S2CEncKey, result.S2CAuthKey)
	if err != nil {
		egressConn.Close()
		return nil, err
	}
	udpDecoder, err := frame.NewUDPFramer(t.cfg.PSK, result.C2SEncKey, result.C2SAuthKey, result.S2CEncKey, result.S2CAuthKey)
	if err != nil {
		egressConn.Close()
		return nil, err
	}

	sess := &udpClientSession{
		clientAddr: cloneUDPAddr(clientAddr),
		egressConn: egressConn,
		framer:     framer,
		udpEncoder: udpEncoder,
		udpDecoder: udpDecoder,
		sendQueue:  make(chan []byte, udpSessionSendQueue),
		done:       make(chan struct{}),
	}
	sess.touch(now)

	if err := sess.framer.WriteFrame(sess.egressConn, encodeTargetAddr(2, t.cfg.TargetAddr)); err != nil {
		egressConn.Close()
		return nil, err
	}

	t.trackCloser(egressConn)
	t.activeConns.Add(1)

	t.udpMu.Lock()
	if existing, ok := t.udpSessions[key]; ok {
		t.udpMu.Unlock()
		existing.touch(time.Now())
		t.untrackCloser(egressConn)
		t.activeConns.Add(-1)
		egressConn.Close()
		return existing, nil
	}
	t.udpSessions[key] = sess
	t.udpMu.Unlock()

	t.wg.Add(1)
	go t.runUDPSendLoop(key, sess)
	t.wg.Add(1)
	go t.runUDPResponseLoop(key, sess)

	return sess, nil
}

func (t *IngressTunnel) oldestUDPSessionKeyLocked() string {
	var oldestKey string
	var oldestSeen int64
	for key, sess := range t.udpSessions {
		seen := sess.lastSeen.Load()
		if oldestKey == "" || seen < oldestSeen {
			oldestKey = key
			oldestSeen = seen
		}
	}
	return oldestKey
}

func (s *udpClientSession) touch(now time.Time) {
	s.lastSeen.Store(now.UnixNano())
}

func (s *udpClientSession) idleFor(now time.Time) time.Duration {
	seen := s.lastSeen.Load()
	if seen == 0 {
		return 0
	}
	return now.Sub(time.Unix(0, seen))
}

func (s *udpClientSession) enqueue(payload []byte) bool {
	s.touch(time.Now())
	select {
	case <-s.done:
		return false
	case s.sendQueue <- payload:
		return true
	default:
		return false
	}
}

func (t *IngressTunnel) runUDPSendLoop(key string, sess *udpClientSession) {
	defer t.wg.Done()
	defer t.closeUDPSession(key)

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-sess.done:
			return
		case payload := <-sess.sendQueue:
			encoded, encErr := sess.udpEncoder.EncodePacket(payload)
			if encErr != nil {
				t.udpDropped.Add(1)
				return
			}
			if err := sess.framer.WriteFrame(sess.egressConn, encoded); err != nil {
				return
			}
		}
	}
}

func (t *IngressTunnel) runUDPResponseLoop(key string, sess *udpClientSession) {
	defer t.wg.Done()
	defer t.closeUDPSession(key)

	for {
		if t.ctx.Err() != nil {
			return
		}
		_ = sess.egressConn.SetReadDeadline(time.Now().Add(udpIdleTimeout))
		encoded, err := sess.framer.ReadFrame(sess.egressConn)
		if len(encoded) > 0 {
			payload, decErr := sess.udpDecoder.DecodePacket(encoded)
			if decErr != nil {
				t.udpDecodeErrors.Add(1)
				if strings.Contains(decErr.Error(), "replay") {
					t.udpReplayErrors.Add(1)
				}
				return
			}
			sess.touch(time.Now())
			if len(payload) > 0 {
				if _, writeErr := t.udpConn.WriteToUDP(payload, sess.clientAddr); writeErr != nil {
					return
				}
				t.bytesDown.Add(int64(len(payload)))
			}
		}
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() && sess.idleFor(time.Now()) < udpIdleTimeout {
				continue
			}
			return
		}
	}
}

func (t *IngressTunnel) runUDPCleanupLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(udpCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case now := <-ticker.C:
			t.closeIdleUDPSessions(now)
		}
	}
}

func (t *IngressTunnel) closeIdleUDPSessions(now time.Time) {
	var expired []string

	t.udpMu.Lock()
	for key, sess := range t.udpSessions {
		if sess.idleFor(now) >= udpIdleTimeout {
			expired = append(expired, key)
		}
	}
	t.udpMu.Unlock()

	for _, key := range expired {
		t.closeUDPSession(key)
	}
}

func (t *IngressTunnel) closeUDPSession(key string) {
	t.udpMu.Lock()
	session, ok := t.udpSessions[key]
	if ok {
		delete(t.udpSessions, key)
	}
	t.udpMu.Unlock()
	if !ok {
		return
	}

	session.closeOnce.Do(func() {
		if session.done != nil {
			close(session.done)
		}
		t.untrackCloser(session.egressConn)
		if session.egressConn != nil {
			_ = session.egressConn.Close()
		}
		t.activeConns.Add(-1)
	})
}

func (t *IngressTunnel) openEgressTunnel() (net.Conn, *frame.TCPFramer, *session.HandshakeResult, error) {
	conn, err := dialNetwork("tcp", t.cfg.EgressAddr)
	if err != nil {
		return nil, nil, nil, err
	}

	result, err := session.ClientHandshake(conn, t.cfg.PSK)
	if err != nil {
		conn.Close()
		return nil, nil, nil, err
	}

	framer, err := frame.NewTCPFramer(t.cfg.PSK, result.C2SEncKey, result.C2SAuthKey, result.S2CEncKey, result.S2CAuthKey, result.Nonce)
	if err != nil {
		conn.Close()
		return nil, nil, nil, err
	}

	return conn, framer, result, nil
}

func (t *IngressTunnel) listenUDP() (*net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr("udp", t.cfg.ListenAddr)
	if err != nil {
		return nil, err
	}
	return net.ListenUDP("udp", addr)
}

func (t *IngressTunnel) trackCloser(c io.Closer) {
	t.closerMu.Lock()
	t.closers[c] = struct{}{}
	t.closerMu.Unlock()
}

func (t *IngressTunnel) untrackCloser(c io.Closer) {
	t.closerMu.Lock()
	delete(t.closers, c)
	t.closerMu.Unlock()
}

func (t *IngressTunnel) closeAllActive() {
	t.closerMu.Lock()
	closers := make([]io.Closer, 0, len(t.closers))
	for c := range t.closers {
		closers = append(closers, c)
	}
	t.closerMu.Unlock()

	for _, c := range closers {
		_ = c.Close()
	}
}

func encodeTargetAddr(proto byte, target string) []byte {
	buf := make([]byte, 1+4+len(target))
	buf[0] = proto
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(target)))
	copy(buf[5:], target)
	return buf
}

func cloneUDPAddr(addr *net.UDPAddr) *net.UDPAddr {
	if addr == nil {
		return nil
	}
	ip := append([]byte(nil), addr.IP...)
	return &net.UDPAddr{
		IP:   ip,
		Port: addr.Port,
		Zone: addr.Zone,
	}
}

func udpDatagramReadSize() int {
	return 64 * 1024
}
