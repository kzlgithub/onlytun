package frame

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	otcrypto "github.com/onlytun/agent/crypto"
	"golang.org/x/crypto/blake2b"
)

func TestTCPFramerWriteReadRoundTrip(t *testing.T) {
	psk := bytes.Repeat([]byte{0x81}, 32)
	clientRandom := []byte("client-random-frame-tcp")
	serverRandom := []byte("server-random-frame-tcp")
	encKey, authKey := otcrypto.DeriveSessionKeys(psk, clientRandom, serverRandom)
	keys := testDirectionalKeys(encKey, authKey)
	nonce := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

	writer, err := NewTCPFramer(psk, keys.c2sEnc, keys.c2sAuth, keys.s2cEnc, keys.s2cAuth, nonce)
	if err != nil {
		t.Fatalf("failed to create writer framer: %v", err)
	}
	reader, err := NewTCPFramer(psk, keys.s2cEnc, keys.s2cAuth, keys.c2sEnc, keys.c2sAuth, nonce)
	if err != nil {
		t.Fatalf("failed to create reader framer: %v", err)
	}

	payload := bytes.Repeat([]byte("onlytun-tcp-payload-"), 8)
	var stream bytes.Buffer
	if err := writer.WriteFrame(&stream, payload); err != nil {
		t.Fatalf("failed to write frame: %v", err)
	}

	got, err := reader.ReadFrame(&stream)
	if err != nil {
		t.Fatalf("failed to read frame: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("decoded payload does not match original payload")
	}
}

func TestTCPFramerStickyPackets(t *testing.T) {
	psk := bytes.Repeat([]byte{0x82}, 32)
	clientRandom := []byte("client-random-frame-sticky")
	serverRandom := []byte("server-random-frame-sticky")
	encKey, authKey := otcrypto.DeriveSessionKeys(psk, clientRandom, serverRandom)
	keys := testDirectionalKeys(encKey, authKey)
	nonce := []byte{11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0}

	writer, err := NewTCPFramer(psk, keys.c2sEnc, keys.c2sAuth, keys.s2cEnc, keys.s2cAuth, nonce)
	if err != nil {
		t.Fatalf("failed to create writer framer: %v", err)
	}
	reader, err := NewTCPFramer(psk, keys.s2cEnc, keys.s2cAuth, keys.c2sEnc, keys.c2sAuth, nonce)
	if err != nil {
		t.Fatalf("failed to create reader framer: %v", err)
	}

	payloads := [][]byte{
		[]byte("frame-one"),
		bytes.Repeat([]byte("frame-two-"), 10),
		[]byte(""),
		[]byte("frame-four"),
	}

	var stream bytes.Buffer
	for _, payload := range payloads {
		if err := writer.WriteFrame(&stream, payload); err != nil {
			t.Fatalf("failed to write frame: %v", err)
		}
	}

	for i, want := range payloads {
		got, err := reader.ReadFrame(&stream)
		if err != nil {
			t.Fatalf("failed to read frame %d: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("frame %d payload mismatch", i)
		}
	}
}

func TestTCPFramerRejectsTampering(t *testing.T) {
	psk := bytes.Repeat([]byte{0x83}, 32)
	clientRandom := []byte("client-random-frame-tamper")
	serverRandom := []byte("server-random-frame-tamper")
	encKey, authKey := otcrypto.DeriveSessionKeys(psk, clientRandom, serverRandom)
	keys := testDirectionalKeys(encKey, authKey)
	nonce := []byte{1, 3, 5, 7, 9, 11, 13, 15, 2, 4, 6, 8}

	writer, err := NewTCPFramer(psk, keys.c2sEnc, keys.c2sAuth, keys.s2cEnc, keys.s2cAuth, nonce)
	if err != nil {
		t.Fatalf("failed to create writer framer: %v", err)
	}
	reader, err := NewTCPFramer(psk, keys.s2cEnc, keys.s2cAuth, keys.c2sEnc, keys.c2sAuth, nonce)
	if err != nil {
		t.Fatalf("failed to create reader framer: %v", err)
	}

	var stream bytes.Buffer
	if err := writer.WriteFrame(&stream, []byte("tamper-check")); err != nil {
		t.Fatalf("failed to write frame: %v", err)
	}

	packet := append([]byte(nil), stream.Bytes()...)
	packet[len(packet)-1] ^= 0x01

	if _, err := reader.ReadFrame(bytes.NewReader(packet)); err == nil {
		t.Fatal("expected tampered frame to fail verification")
	}
}

func TestTCPFramerEmptyPayload(t *testing.T) {
	psk := bytes.Repeat([]byte{0x84}, 32)
	clientRandom := []byte("client-random-frame-empty")
	serverRandom := []byte("server-random-frame-empty")
	encKey, authKey := otcrypto.DeriveSessionKeys(psk, clientRandom, serverRandom)
	keys := testDirectionalKeys(encKey, authKey)
	nonce := []byte{9, 9, 8, 8, 7, 7, 6, 6, 5, 5, 4, 4}

	writer, err := NewTCPFramer(psk, keys.c2sEnc, keys.c2sAuth, keys.s2cEnc, keys.s2cAuth, nonce)
	if err != nil {
		t.Fatalf("failed to create writer framer: %v", err)
	}
	reader, err := NewTCPFramer(psk, keys.s2cEnc, keys.s2cAuth, keys.c2sEnc, keys.c2sAuth, nonce)
	if err != nil {
		t.Fatalf("failed to create reader framer: %v", err)
	}

	var stream bytes.Buffer
	if err := writer.WriteFrame(&stream, nil); err != nil {
		t.Fatalf("failed to write empty frame: %v", err)
	}

	got, err := reader.ReadFrame(&stream)
	if err != nil {
		t.Fatalf("failed to read empty frame: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty payload, got %d bytes", len(got))
	}
}

type shortWriter struct {
	w        io.Writer
	maxBytes int
}

func (sw *shortWriter) Write(p []byte) (int, error) {
	if len(p) > sw.maxBytes {
		p = p[:sw.maxBytes]
	}
	return sw.w.Write(p)
}

func TestWriteFrameShortWriter(t *testing.T) {
	psk := bytes.Repeat([]byte{0x85}, 32)
	clientRandom := []byte("client-random-short-writer")
	serverRandom := []byte("server-random-short-writer")
	encKey, authKey := otcrypto.DeriveSessionKeys(psk, clientRandom, serverRandom)
	keys := testDirectionalKeys(encKey, authKey)
	nonce := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

	writer, err := NewTCPFramer(psk, keys.c2sEnc, keys.c2sAuth, keys.s2cEnc, keys.s2cAuth, nonce)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	reader, err := NewTCPFramer(psk, keys.s2cEnc, keys.s2cAuth, keys.c2sEnc, keys.c2sAuth, nonce)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}

	payload := bytes.Repeat([]byte("short-write-payload-"), 10)

	var buf bytes.Buffer
	sw := &shortWriter{w: &buf, maxBytes: 3}

	if err := writer.WriteFrame(sw, payload); err != nil {
		t.Fatalf("WriteFrame failed with short writer: %v", err)
	}

	got, err := reader.ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("payload mismatch after short write")
	}
}

type directionalKeys struct {
	c2sEnc  []byte
	c2sAuth []byte
	s2cEnc  []byte
	s2cAuth []byte
}

func testDirectionalKeys(encKey, authKey []byte) directionalKeys {
	return directionalKeys{
		c2sEnc:  deriveTestKey(encKey, "OnlyTun-c2s-enc-v1"),
		c2sAuth: deriveTestKey(authKey, "OnlyTun-c2s-auth-v1"),
		s2cEnc:  deriveTestKey(encKey, "OnlyTun-s2c-enc-v1"),
		s2cAuth: deriveTestKey(authKey, "OnlyTun-s2c-auth-v1"),
	}
}

func deriveTestKey(root []byte, label string) []byte {
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
