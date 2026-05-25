package tunnel

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	otcrypto "github.com/onlytun/agent/crypto"
	"github.com/onlytun/agent/frame"
	"github.com/onlytun/agent/session"
	"golang.org/x/crypto/blake2b"
)

func TestOnlyTunTCPEndToEnd(t *testing.T) {
	psk := bytes.Repeat([]byte{0x61}, 32)

	targetAddr, stopTarget := startTCPEchoServer(t)
	defer stopTarget()

	egress := NewEgressTunnel(EgressConfig{
		ListenAddr: "127.0.0.1:0",
		PSK:        psk,
	})
	if err := egress.Start(); err != nil {
		t.Fatalf("start egress: %v", err)
	}
	defer egress.Stop()

	ingress := NewIngressTunnel(IngressConfig{
		ListenAddr: "127.0.0.1:0",
		Protocol:   "tcp",
		EgressAddr: egress.listener.Addr().String(),
		TargetAddr: targetAddr,
		PSK:        psk,
		RuleID:     "tcp-e2e",
	})
	if err := ingress.Start(); err != nil {
		t.Fatalf("start ingress: %v", err)
	}
	defer ingress.Stop()

	clientConn, err := net.Dial("tcp", ingress.tcpListener.Addr().String())
	if err != nil {
		t.Fatalf("dial ingress tcp: %v", err)
	}
	defer clientConn.Close()

	payload := bytes.Repeat([]byte("onlytun-tcp-e2e-"), 8)
	if _, err := clientConn.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	reply := make([]byte, len(payload))
	if _, err := io.ReadFull(clientConn, reply); err != nil {
		t.Fatalf("read reply: %v", err)
	}
	if !bytes.Equal(reply, payload) {
		t.Fatal("tcp reply mismatch")
	}

	waitForCondition(t, func() bool {
		stats := ingress.GetStats()
		return stats.BytesUp >= int64(len(payload)) && stats.BytesDown >= int64(len(payload))
	})
	waitForCondition(t, func() bool {
		stats := egress.GetStats()
		return stats.BytesUp >= int64(len(payload)) && stats.BytesDown >= int64(len(payload))
	})

	_ = clientConn.Close()
	waitForCondition(t, func() bool {
		return ingress.GetStats().ActiveConns == 0 && egress.GetStats().ActiveConns == 0
	})
}

func TestOnlyTunUDPEndToEnd(t *testing.T) {
	psk := bytes.Repeat([]byte{0x62}, 32)

	targetAddr, stopTarget := startUDPEchoServer(t)
	defer stopTarget()

	egress := NewEgressTunnel(EgressConfig{
		ListenAddr: "127.0.0.1:0",
		PSK:        psk,
	})
	if err := egress.Start(); err != nil {
		t.Fatalf("start egress: %v", err)
	}
	defer egress.Stop()

	ingress := NewIngressTunnel(IngressConfig{
		ListenAddr: "127.0.0.1:0",
		Protocol:   "udp",
		EgressAddr: egress.listener.Addr().String(),
		TargetAddr: targetAddr,
		PSK:        psk,
		RuleID:     "udp-e2e",
	})
	if err := ingress.Start(); err != nil {
		t.Fatalf("start ingress: %v", err)
	}
	defer ingress.Stop()

	remoteAddr, err := net.ResolveUDPAddr("udp", ingress.udpConn.LocalAddr().String())
	if err != nil {
		t.Fatalf("resolve ingress udp: %v", err)
	}
	clientConn, err := net.DialUDP("udp", nil, remoteAddr)
	if err != nil {
		t.Fatalf("dial ingress udp: %v", err)
	}
	defer clientConn.Close()

	payload := bytes.Repeat([]byte("onlytun-udp-e2e-"), 8)
	if _, err := clientConn.Write(payload); err != nil {
		t.Fatalf("write udp payload: %v", err)
	}

	reply := make([]byte, len(payload))
	_ = clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := clientConn.Read(reply)
	if err != nil {
		t.Fatalf("read udp reply: %v", err)
	}
	reply = reply[:n]
	if !bytes.Equal(reply, payload) {
		t.Fatal("udp reply mismatch")
	}

	waitForCondition(t, func() bool {
		stats := ingress.GetStats()
		return stats.BytesUp >= int64(len(payload)) && stats.BytesDown >= int64(len(payload))
	})
	waitForCondition(t, func() bool {
		stats := egress.GetStats()
		return stats.BytesUp >= int64(len(payload)) && stats.BytesDown >= int64(len(payload))
	})

	_ = clientConn.Close()
	ingress.closeAllActive()
	egress.closeAllActive()
	waitForCondition(t, func() bool {
		return ingress.GetStats().ActiveConns == 0 && egress.GetStats().ActiveConns == 0
	})
}

func TestOnlyTunTCPWrongPSKClosesClient(t *testing.T) {
	egressPSK := bytes.Repeat([]byte{0x63}, 32)
	ingressPSK := bytes.Repeat([]byte{0x64}, 32)

	targetAddr, stopTarget := startTCPEchoServer(t)
	defer stopTarget()

	egress := NewEgressTunnel(EgressConfig{
		ListenAddr: "127.0.0.1:0",
		PSK:        egressPSK,
	})
	if err := egress.Start(); err != nil {
		t.Fatalf("start egress: %v", err)
	}
	defer egress.Stop()

	ingress := NewIngressTunnel(IngressConfig{
		ListenAddr: "127.0.0.1:0",
		Protocol:   "tcp",
		EgressAddr: egress.listener.Addr().String(),
		TargetAddr: targetAddr,
		PSK:        ingressPSK,
		RuleID:     "tcp-wrong-psk",
	})
	if err := ingress.Start(); err != nil {
		t.Fatalf("start ingress: %v", err)
	}
	defer ingress.Stop()

	clientConn, err := net.Dial("tcp", ingress.tcpListener.Addr().String())
	if err != nil {
		t.Fatalf("dial ingress tcp: %v", err)
	}
	defer clientConn.Close()

	_, _ = clientConn.Write([]byte("hello"))
	expectConnClosed(t, clientConn)
	waitForCondition(t, func() bool {
		return ingress.GetStats().ActiveConns == 0 && egress.GetStats().ActiveConns == 0
	})
}

func TestOnlyTunTCPTargetDialFailureClosesClient(t *testing.T) {
	psk := bytes.Repeat([]byte{0x65}, 32)
	targetAddr := reserveTCPAddr(t)

	egress := NewEgressTunnel(EgressConfig{
		ListenAddr: "127.0.0.1:0",
		PSK:        psk,
	})
	if err := egress.Start(); err != nil {
		t.Fatalf("start egress: %v", err)
	}
	defer egress.Stop()

	ingress := NewIngressTunnel(IngressConfig{
		ListenAddr: "127.0.0.1:0",
		Protocol:   "tcp",
		EgressAddr: egress.listener.Addr().String(),
		TargetAddr: targetAddr,
		PSK:        psk,
		RuleID:     "tcp-target-fail",
	})
	if err := ingress.Start(); err != nil {
		t.Fatalf("start ingress: %v", err)
	}
	defer ingress.Stop()

	clientConn, err := net.Dial("tcp", ingress.tcpListener.Addr().String())
	if err != nil {
		t.Fatalf("dial ingress tcp: %v", err)
	}
	defer clientConn.Close()

	_, _ = clientConn.Write([]byte("hello"))
	expectConnClosed(t, clientConn)
	waitForCondition(t, func() bool {
		return ingress.GetStats().ActiveConns == 0 && egress.GetStats().ActiveConns == 0
	})
}

func TestOnlyTunUDPReplayPacketClosesTunnel(t *testing.T) {
	psk := bytes.Repeat([]byte{0x66}, 32)

	targetAddr, stopTarget := startUDPEchoServer(t)
	defer stopTarget()

	egress := NewEgressTunnel(EgressConfig{
		ListenAddr: "127.0.0.1:0",
		PSK:        psk,
	})
	if err := egress.Start(); err != nil {
		t.Fatalf("start egress: %v", err)
	}
	defer egress.Stop()

	conn, err := net.Dial("tcp", egress.listener.Addr().String())
	if err != nil {
		t.Fatalf("dial egress: %v", err)
	}
	defer conn.Close()

	result, err := session.ClientHandshake(conn, psk)
	if err != nil {
		t.Fatalf("client handshake: %v", err)
	}

	outerFramer, err := frame.NewTCPFramer(psk, result.C2SEncKey, result.C2SAuthKey, result.S2CEncKey, result.S2CAuthKey, result.Nonce)
	if err != nil {
		t.Fatalf("outer framer: %v", err)
	}
	udpWriter, err := frame.NewUDPFramer(psk, result.C2SEncKey, result.C2SAuthKey, result.S2CEncKey, result.S2CAuthKey)
	if err != nil {
		t.Fatalf("udp writer: %v", err)
	}
	udpReader, err := frame.NewUDPFramer(psk, result.C2SEncKey, result.C2SAuthKey, result.S2CEncKey, result.S2CAuthKey)
	if err != nil {
		t.Fatalf("udp reader: %v", err)
	}

	if err := outerFramer.WriteFrame(conn, encodeTargetAddr(2, targetAddr)); err != nil {
		t.Fatalf("write target frame: %v", err)
	}

	payload := []byte("udp-replay-check")
	packet, err := udpWriter.EncodePacket(payload)
	if err != nil {
		t.Fatalf("encode udp payload: %v", err)
	}
	if err := outerFramer.WriteFrame(conn, packet); err != nil {
		t.Fatalf("write first udp frame: %v", err)
	}

	encodedReply, err := outerFramer.ReadFrame(conn)
	if err != nil {
		t.Fatalf("read echoed udp frame: %v", err)
	}
	reply, err := udpReader.DecodePacket(encodedReply)
	if err != nil {
		t.Fatalf("decode echoed udp frame: %v", err)
	}
	if !bytes.Equal(reply, payload) {
		t.Fatal("udp echoed payload mismatch")
	}

	if err := outerFramer.WriteFrame(conn, packet); err != nil {
		t.Fatalf("write replayed udp frame: %v", err)
	}

	expectConnClosed(t, conn)
	waitForCondition(t, func() bool {
		return egress.GetStats().ActiveConns == 0
	})
}

func TestEgressBridgeUDPTunnelClosesOnDecodeError(t *testing.T) {
	psk := bytes.Repeat([]byte{0x71}, 32)
	keys := testTunnelDirectionalKeys(psk, []byte("egress-client-random"), []byte("egress-server-random"))
	nonce := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

	clientFramer, err := frame.NewTCPFramer(psk, keys.c2sEnc, keys.c2sAuth, keys.s2cEnc, keys.s2cAuth, nonce)
	if err != nil {
		t.Fatalf("client framer: %v", err)
	}
	serverFramer, err := frame.NewTCPFramer(psk, keys.s2cEnc, keys.s2cAuth, keys.c2sEnc, keys.c2sAuth, nonce)
	if err != nil {
		t.Fatalf("server framer: %v", err)
	}

	udpDecoder, err := frame.NewUDPFramer(psk, keys.s2cEnc, keys.s2cAuth, keys.c2sEnc, keys.c2sAuth)
	if err != nil {
		t.Fatalf("udp decoder: %v", err)
	}
	udpEncoder, err := frame.NewUDPFramer(psk, keys.s2cEnc, keys.s2cAuth, keys.c2sEnc, keys.c2sAuth)
	if err != nil {
		t.Fatalf("udp encoder: %v", err)
	}

	wrongPSK := bytes.Repeat([]byte{0x72}, 32)
	badKeys := testTunnelDirectionalKeys(wrongPSK, []byte("bad-client-random"), []byte("bad-server-random"))
	badPacketEncoder, err := frame.NewUDPFramer(psk, badKeys.c2sEnc, badKeys.c2sAuth, badKeys.s2cEnc, badKeys.s2cAuth)
	if err != nil {
		t.Fatalf("bad packet encoder: %v", err)
	}

	tunnelClient, tunnelServer := net.Pipe()
	targetServer, targetPeer := net.Pipe()
	defer tunnelClient.Close()
	defer targetPeer.Close()

	egress := NewEgressTunnel(EgressConfig{PSK: psk})
	done := make(chan struct{})
	go func() {
		egress.bridgeUDPTunnel(tunnelServer, targetServer, serverFramer, udpDecoder, udpEncoder)
		close(done)
	}()

	badPacket, err := badPacketEncoder.EncodePacket([]byte("bad-inner-payload"))
	if err != nil {
		t.Fatalf("encode bad packet: %v", err)
	}
	if err := clientFramer.WriteFrame(tunnelClient, badPacket); err != nil {
		t.Fatalf("write outer frame: %v", err)
	}

	expectConnClosed(t, tunnelClient)
	expectConnClosed(t, targetPeer)
	waitForDone(t, done)
}

func TestIngressUDPResponseLoopClosesSessionOnDecodeError(t *testing.T) {
	psk := bytes.Repeat([]byte{0x73}, 32)
	keys := testTunnelDirectionalKeys(psk, []byte("ingress-client-random"), []byte("ingress-server-random"))
	nonce := []byte{11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0}

	ingressFramer, err := frame.NewTCPFramer(psk, keys.c2sEnc, keys.c2sAuth, keys.s2cEnc, keys.s2cAuth, nonce)
	if err != nil {
		t.Fatalf("ingress framer: %v", err)
	}
	egressFramer, err := frame.NewTCPFramer(psk, keys.s2cEnc, keys.s2cAuth, keys.c2sEnc, keys.c2sAuth, nonce)
	if err != nil {
		t.Fatalf("egress framer: %v", err)
	}

	udpDecoder, err := frame.NewUDPFramer(psk, keys.c2sEnc, keys.c2sAuth, keys.s2cEnc, keys.s2cAuth)
	if err != nil {
		t.Fatalf("udp decoder: %v", err)
	}

	wrongPSK := bytes.Repeat([]byte{0x74}, 32)
	badKeys := testTunnelDirectionalKeys(wrongPSK, []byte("other-client-random"), []byte("other-server-random"))
	badPacketEncoder, err := frame.NewUDPFramer(psk, badKeys.s2cEnc, badKeys.s2cAuth, badKeys.c2sEnc, badKeys.c2sAuth)
	if err != nil {
		t.Fatalf("bad packet encoder: %v", err)
	}

	udpListen, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer udpListen.Close()

	udpClient, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen udp client: %v", err)
	}
	defer udpClient.Close()

	egressClient, egressServer := net.Pipe()
	defer egressClient.Close()

	tunnel := NewIngressTunnel(IngressConfig{PSK: psk})
	tunnel.udpConn = udpListen
	key := udpClient.LocalAddr().String()
	sess := &udpClientSession{
		clientAddr: udpClient.LocalAddr().(*net.UDPAddr),
		egressConn: egressServer,
		framer:     ingressFramer,
		udpDecoder: udpDecoder,
	}

	tunnel.udpMu.Lock()
	tunnel.udpSessions[key] = sess
	tunnel.udpMu.Unlock()
	tunnel.trackCloser(egressServer)
	tunnel.activeConns.Store(1)

	done := make(chan struct{})
	tunnel.wg.Add(1)
	go func() {
		tunnel.runUDPResponseLoop(key, sess)
		close(done)
	}()

	badPacket, err := badPacketEncoder.EncodePacket([]byte("bad-response"))
	if err != nil {
		t.Fatalf("encode bad packet: %v", err)
	}
	if err := egressFramer.WriteFrame(egressClient, badPacket); err != nil {
		t.Fatalf("write outer frame: %v", err)
	}

	waitForCondition(t, func() bool {
		tunnel.udpMu.Lock()
		defer tunnel.udpMu.Unlock()
		_, ok := tunnel.udpSessions[key]
		return !ok
	})
	expectConnClosed(t, egressClient)
	waitForDone(t, done)
}

type tunnelDirectionalKeys struct {
	c2sEnc  []byte
	c2sAuth []byte
	s2cEnc  []byte
	s2cAuth []byte
}

func testTunnelDirectionalKeys(psk, clientRandom, serverRandom []byte) tunnelDirectionalKeys {
	encKey, authKey := otcrypto.DeriveSessionKeys(psk, clientRandom, serverRandom)
	return tunnelDirectionalKeys{
		c2sEnc:  deriveTunnelKey(encKey, "OnlyTun-c2s-enc-v1"),
		c2sAuth: deriveTunnelKey(authKey, "OnlyTun-c2s-auth-v1"),
		s2cEnc:  deriveTunnelKey(encKey, "OnlyTun-s2c-enc-v1"),
		s2cAuth: deriveTunnelKey(authKey, "OnlyTun-s2c-auth-v1"),
	}
}

func deriveTunnelKey(root []byte, label string) []byte {
	h, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	for _, part := range [][]byte{root, []byte(label)} {
		var size [4]byte
		binary.LittleEndian.PutUint32(size[:], uint32(len(part)))
		_, _ = h.Write(size[:])
		_, _ = h.Write(part)
	}
	return h.Sum(nil)
}

func expectConnClosed(t *testing.T, conn net.Conn) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var buf [1]byte
	_, err := conn.Read(buf[:])
	if err == nil {
		t.Fatal("expected connection to close")
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatalf("timed out waiting for connection close: %v", err)
	}
}

func waitForDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for goroutine to exit")
	}
}

func waitForCondition(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

func startTCPEchoServer(t *testing.T) (string, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp echo: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()

	return ln.Addr().String(), func() {
		_ = ln.Close()
		waitForDone(t, done)
	}
}

func startUDPEchoServer(t *testing.T) (string, func()) {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("listen udp echo: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 65535)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = conn.WriteToUDP(buf[:n], addr)
		}
	}()

	return conn.LocalAddr().String(), func() {
		_ = conn.Close()
		waitForDone(t, done)
	}
}

func reserveTCPAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp addr: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}
