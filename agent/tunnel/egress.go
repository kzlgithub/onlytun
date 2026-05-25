package tunnel

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/onlytun/agent/frame"
	"github.com/onlytun/agent/session"
)

var errInvalidTargetFrame = errors.New("tunnel: invalid target frame")

// EgressConfig 出口机配置。
type EgressConfig struct {
	ListenAddr string
	PSK        []byte
}

// EgressTunnel 出口机隧道。
type EgressTunnel struct {
	cfg    EgressConfig
	ctx    context.Context
	cancel context.CancelFunc

	startMu sync.Mutex
	wg      sync.WaitGroup

	listener net.Listener

	activeConns atomic.Int64
	bytesUp     atomic.Int64
	bytesDown   atomic.Int64

	closerMu sync.Mutex
	closers  map[io.Closer]struct{}
}

// NewEgressTunnel 创建出口机隧道。
func NewEgressTunnel(cfg EgressConfig) *EgressTunnel {
	ctx, cancel := context.WithCancel(context.Background())
	return &EgressTunnel{
		cfg:     cfg,
		ctx:     ctx,
		cancel:  cancel,
		closers: make(map[io.Closer]struct{}),
	}
}

// Start 开始监听，非阻塞。
func (t *EgressTunnel) Start() error {
	t.startMu.Lock()
	defer t.startMu.Unlock()

	if t.listener != nil {
		return errIngressStarted
	}

	ln, err := net.Listen("tcp", t.cfg.ListenAddr)
	if err != nil {
		return err
	}

	t.listener = ln
	t.wg.Add(1)
	go t.acceptLoop()
	return nil
}

// Stop 停止监听。
func (t *EgressTunnel) Stop() {
	t.cancel()

	t.startMu.Lock()
	if t.listener != nil {
		_ = t.listener.Close()
	}
	t.startMu.Unlock()

	t.closeAllActive()

	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(stopWaitTimeout):
	}
}

// GetStats 获取统计数据。
func (t *EgressTunnel) GetStats() Stats {
	return Stats{
		ActiveConns: t.activeConns.Load(),
		BytesUp:     t.bytesUp.Load(),
		BytesDown:   t.bytesDown.Load(),
	}
}

func (t *EgressTunnel) acceptLoop() {
	defer t.wg.Done()

	for {
		conn, err := t.listener.Accept()
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

		t.wg.Add(1)
		go t.handleTunnelConn(conn)
	}
}

func (t *EgressTunnel) handleTunnelConn(tunnelConn net.Conn) {
	defer t.wg.Done()

	result, err := session.ServerHandshake(tunnelConn, t.cfg.PSK)
	if err != nil {
		_ = tunnelConn.Close()
		return
	}

	framer, err := frame.NewTCPFramer(t.cfg.PSK, result.S2CEncKey, result.S2CAuthKey, result.C2SEncKey, result.C2SAuthKey, result.Nonce)
	if err != nil {
		_ = tunnelConn.Close()
		return
	}

	targetFrame, err := framer.ReadFrame(tunnelConn)
	if err != nil {
		_ = tunnelConn.Close()
		return
	}

	proto, targetAddr, err := decodeTargetAddr(targetFrame)
	if err != nil {
		_ = tunnelConn.Close()
		return
	}

	var network string
	switch proto {
	case 1:
		network = "tcp"
	case 2:
		network = "udp"
	default:
		_ = tunnelConn.Close()
		return
	}

	targetConn, err := net.Dial(network, targetAddr)
	if err != nil {
		_ = tunnelConn.Close()
		return
	}

	t.activeConns.Add(1)
	t.trackCloser(tunnelConn)
	t.trackCloser(targetConn)
	defer func() {
		t.untrackCloser(tunnelConn)
		t.untrackCloser(targetConn)
		t.activeConns.Add(-1)
	}()

	if proto == 1 {
		t.bridgeTunnel(tunnelConn, targetConn, framer)
	} else {
		udpDecoder, err := frame.NewUDPFramer(t.cfg.PSK, result.S2CEncKey, result.S2CAuthKey, result.C2SEncKey, result.C2SAuthKey)
		if err != nil {
			return
		}
		udpEncoder, err := frame.NewUDPFramer(t.cfg.PSK, result.S2CEncKey, result.S2CAuthKey, result.C2SEncKey, result.C2SAuthKey)
		if err != nil {
			return
		}
		t.bridgeUDPTunnel(tunnelConn, targetConn, framer, udpDecoder, udpEncoder)
	}
}

func (t *EgressTunnel) bridgeTunnel(tunnelConn net.Conn, targetConn net.Conn, framer *frame.TCPFramer) {
	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			_ = tunnelConn.Close()
			_ = targetConn.Close()
		})
	}

	errCh := make(chan error, 2)

	go func() {
		for {
			payload, err := framer.ReadFrame(tunnelConn)
			if len(payload) > 0 {
				if _, writeErr := targetConn.Write(payload); writeErr != nil {
					errCh <- writeErr
					return
				}
				t.bytesUp.Add(int64(len(payload)))
			}
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, copyBufferSize)
		for {
			n, err := targetConn.Read(buf)
			if n > 0 {
				if writeErr := framer.WriteFrame(tunnelConn, buf[:n]); writeErr != nil {
					errCh <- writeErr
					return
				}
				t.bytesDown.Add(int64(n))
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

func (t *EgressTunnel) trackCloser(c io.Closer) {
	t.closerMu.Lock()
	t.closers[c] = struct{}{}
	t.closerMu.Unlock()
}

func (t *EgressTunnel) untrackCloser(c io.Closer) {
	t.closerMu.Lock()
	delete(t.closers, c)
	t.closerMu.Unlock()
}

func (t *EgressTunnel) closeAllActive() {
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

func (t *EgressTunnel) bridgeUDPTunnel(tunnelConn, targetConn net.Conn, framer *frame.TCPFramer, udpDecoder, udpEncoder *frame.UDPFramer) {
	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			_ = tunnelConn.Close()
			_ = targetConn.Close()
		})
	}

	errCh := make(chan error, 2)

	// tunnel → UDP target
	go func() {
		for {
			encoded, err := framer.ReadFrame(tunnelConn)
			if len(encoded) > 0 {
				payload, decErr := udpDecoder.DecodePacket(encoded)
				if decErr != nil {
					errCh <- decErr
					return
				}
				if len(payload) > 0 {
					if _, writeErr := targetConn.Write(payload); writeErr != nil {
						errCh <- writeErr
						return
					}
					t.bytesUp.Add(int64(len(payload)))
				}
			}
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	// UDP target → tunnel
	go func() {
		buf := make([]byte, 65535)
		for {
			n, err := targetConn.Read(buf)
			if n > 0 {
				encoded, encErr := udpEncoder.EncodePacket(buf[:n])
				if encErr == nil {
					if writeErr := framer.WriteFrame(tunnelConn, encoded); writeErr != nil {
						errCh <- writeErr
						return
					}
					t.bytesDown.Add(int64(n))
				}
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

func decodeTargetAddr(framePayload []byte) (byte, string, error) {
	if len(framePayload) < 5 {
		return 0, "", errInvalidTargetFrame
	}

	proto := framePayload[0]
	n := int(binary.BigEndian.Uint32(framePayload[1:5]))
	if n == 0 || 5+n > len(framePayload) {
		return 0, "", errInvalidTargetFrame
	}

	return proto, string(framePayload[5 : 5+n]), nil
}
