package session

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestClientServerHandshakeRoundTrip(t *testing.T) {
	psk := bytes.Repeat([]byte{0x55}, 32)
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	serverResultCh := make(chan *HandshakeResult, 1)
	serverErrCh := make(chan error, 1)
	go func() {
		result, err := ServerHandshake(serverConn, psk)
		serverResultCh <- result
		serverErrCh <- err
	}()

	clientResult, clientErr := ClientHandshake(clientConn, psk)
	serverResult := <-serverResultCh
	serverErr := <-serverErrCh

	if clientErr != nil {
		t.Fatalf("client handshake failed: %v", clientErr)
	}
	if serverErr != nil {
		t.Fatalf("server handshake failed: %v", serverErr)
	}
	if !bytes.Equal(clientResult.EncKey, serverResult.EncKey) {
		t.Fatal("enc keys do not match")
	}
	if !bytes.Equal(clientResult.AuthKey, serverResult.AuthKey) {
		t.Fatal("auth keys do not match")
	}
	if !bytes.Equal(clientResult.Nonce, serverResult.Nonce) {
		t.Fatal("nonces do not match")
	}
	if !bytes.Equal(clientResult.C2SEncKey, serverResult.C2SEncKey) {
		t.Fatal("c2s enc keys do not match")
	}
	if !bytes.Equal(clientResult.C2SAuthKey, serverResult.C2SAuthKey) {
		t.Fatal("c2s auth keys do not match")
	}
	if !bytes.Equal(clientResult.S2CEncKey, serverResult.S2CEncKey) {
		t.Fatal("s2c enc keys do not match")
	}
	if !bytes.Equal(clientResult.S2CAuthKey, serverResult.S2CAuthKey) {
		t.Fatal("s2c auth keys do not match")
	}
	if bytes.Equal(clientResult.C2SEncKey, clientResult.S2CEncKey) {
		t.Fatal("directional encryption keys should differ")
	}
	if bytes.Equal(clientResult.C2SAuthKey, clientResult.S2CAuthKey) {
		t.Fatal("directional auth keys should differ")
	}
}

func TestClientHelloPacketSize(t *testing.T) {
	psk := bytes.Repeat([]byte{0x01}, 32)
	clientRandom := bytes.Repeat([]byte{0x02}, 32)
	padding := bytes.Repeat([]byte{0x03}, 24)

	hello, err := buildClientHelloPacket(psk, clientRandom, 1_000_000, padding)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hello) != clientHelloSize {
		t.Fatalf("expected hello size %d, got %d", clientHelloSize, len(hello))
	}
}

func TestClientHandshakeWrongPSKClosesConnection(t *testing.T) {
	serverPSK := bytes.Repeat([]byte{0x56}, 32)
	clientPSK := bytes.Repeat([]byte{0x57}, 32)
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	serverErrCh := make(chan error, 1)
	go func() {
		_, err := ServerHandshake(serverConn, serverPSK)
		serverErrCh <- err
	}()

	_, clientErr := ClientHandshake(clientConn, clientPSK)
	_ = clientConn.Close()
	serverErr := <-serverErrCh

	if clientErr == nil {
		t.Fatal("expected client handshake to fail with wrong psk")
	}
	if serverErr == nil {
		t.Fatal("expected server handshake to fail with wrong psk")
	}
}

func TestServerHandshakeRejectsExpiredReplay(t *testing.T) {
	psk := bytes.Repeat([]byte{0x58}, 32)
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	oldNow := nowFunc
	nowFunc = func() time.Time {
		return time.Unix(1_700_000_000, 0)
	}
	defer func() { nowFunc = oldNow }()

	clientRandom := bytes.Repeat([]byte{0x42}, 32)
	padding := bytes.Repeat([]byte{0x24}, 24)
	staleTimestamp := uint64(nowFunc().Add(-2 * allowedTimeSkew).Unix())
	hello, err := buildClientHelloPacket(psk, clientRandom, staleTimestamp, padding)
	if err != nil {
		t.Fatalf("failed to build stale client hello: %v", err)
	}

	serverErrCh := make(chan error, 1)
	go func() {
		_, err := ServerHandshake(serverConn, psk)
		serverErrCh <- err
	}()

	if _, err := clientConn.Write(hello); err != nil {
		t.Fatalf("failed to write stale hello: %v", err)
	}

	serverErr := <-serverErrCh
	if !errors.Is(serverErr, errInvalidTimestamp) {
		t.Fatalf("expected invalid timestamp error, got %v", serverErr)
	}

	buf := make([]byte, 1)
	_, readErr := clientConn.Read(buf)
	if !errors.Is(readErr, io.EOF) {
		t.Fatalf("expected connection to close after stale replay, got %v", readErr)
	}
}

type replayRecordConn struct {
	net.Conn
	mu         sync.Mutex
	firstWrite []byte
	wrote      bool
}

func (r *replayRecordConn) Write(p []byte) (int, error) {
	r.mu.Lock()
	if !r.wrote {
		r.firstWrite = append([]byte(nil), p...)
		r.wrote = true
	}
	r.mu.Unlock()
	return r.Conn.Write(p)
}

func TestReplayHandshakeRejected(t *testing.T) {
	psk := bytes.Repeat([]byte{0x5b}, 32)

	clientConn1, serverConn1 := net.Pipe()
	recConn := &replayRecordConn{Conn: clientConn1}

	serverErrCh := make(chan error, 1)
	go func() {
		_, err := ServerHandshake(serverConn1, psk)
		serverErrCh <- err
	}()

	if _, err := ClientHandshake(recConn, psk); err != nil {
		t.Fatalf("first handshake failed: %v", err)
	}
	if err := <-serverErrCh; err != nil {
		t.Fatalf("server first handshake failed: %v", err)
	}
	clientConn1.Close()
	serverConn1.Close()

	capturedHello := recConn.firstWrite
	if len(capturedHello) != clientHelloSize {
		t.Fatalf("captured hello has unexpected size: %d", len(capturedHello))
	}

	clientConn2, serverConn2 := net.Pipe()
	defer clientConn2.Close()

	serverErrCh2 := make(chan error, 1)
	go func() {
		_, err := ServerHandshake(serverConn2, psk)
		serverErrCh2 <- err
		serverConn2.Close()
	}()

	if _, err := clientConn2.Write(capturedHello); err != nil {
		t.Fatalf("replay write failed: %v", err)
	}

	serverErr := <-serverErrCh2
	if serverErr == nil {
		t.Fatal("expected server to reject replayed handshake")
	}
}
