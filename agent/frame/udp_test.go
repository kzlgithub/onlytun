package frame

import (
	"bytes"
	"testing"

	otcrypto "github.com/onlytun/agent/crypto"
)

func TestUDPFramerEncodeDecodeRoundTrip(t *testing.T) {
	psk := bytes.Repeat([]byte{0x91}, 32)
	clientRandom := []byte("client-random-frame-udp")
	serverRandom := []byte("server-random-frame-udp")
	encKey, authKey := otcrypto.DeriveSessionKeys(psk, clientRandom, serverRandom)
	keys := testDirectionalKeys(encKey, authKey)

	enc, err := NewUDPFramer(psk, keys.c2sEnc, keys.c2sAuth, keys.s2cEnc, keys.s2cAuth)
	if err != nil {
		t.Fatalf("failed to create encoder: %v", err)
	}
	dec, err := NewUDPFramer(psk, keys.s2cEnc, keys.s2cAuth, keys.c2sEnc, keys.c2sAuth)
	if err != nil {
		t.Fatalf("failed to create decoder: %v", err)
	}

	payload := bytes.Repeat([]byte("onlytun-udp-payload-"), 12)
	packet, err := enc.EncodePacket(payload)
	if err != nil {
		t.Fatalf("failed to encode packet: %v", err)
	}

	got, err := dec.DecodePacket(packet)
	if err != nil {
		t.Fatalf("failed to decode packet: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("decoded payload mismatch")
	}
}

func TestUDPFramerRejectsReplay(t *testing.T) {
	psk := bytes.Repeat([]byte{0x92}, 32)
	clientRandom := []byte("client-random-frame-udp-replay")
	serverRandom := []byte("server-random-frame-udp-replay")
	encKey, authKey := otcrypto.DeriveSessionKeys(psk, clientRandom, serverRandom)
	keys := testDirectionalKeys(encKey, authKey)

	enc, _ := NewUDPFramer(psk, keys.c2sEnc, keys.c2sAuth, keys.s2cEnc, keys.s2cAuth)
	dec, _ := NewUDPFramer(psk, keys.s2cEnc, keys.s2cAuth, keys.c2sEnc, keys.c2sAuth)

	packet, err := enc.EncodePacket([]byte("replay-check"))
	if err != nil {
		t.Fatalf("failed to encode packet: %v", err)
	}
	if _, err := dec.DecodePacket(packet); err != nil {
		t.Fatalf("first decode failed: %v", err)
	}
	if _, err := dec.DecodePacket(packet); err == nil {
		t.Fatal("expected replayed packet to be rejected")
	}
}

func TestUDPFramerRejectsTampering(t *testing.T) {
	psk := bytes.Repeat([]byte{0x93}, 32)
	clientRandom := []byte("client-random-frame-udp-tamper")
	serverRandom := []byte("server-random-frame-udp-tamper")
	encKey, authKey := otcrypto.DeriveSessionKeys(psk, clientRandom, serverRandom)
	keys := testDirectionalKeys(encKey, authKey)

	enc, _ := NewUDPFramer(psk, keys.c2sEnc, keys.c2sAuth, keys.s2cEnc, keys.s2cAuth)
	dec, _ := NewUDPFramer(psk, keys.s2cEnc, keys.s2cAuth, keys.c2sEnc, keys.c2sAuth)

	packet, err := enc.EncodePacket([]byte("tamper-check"))
	if err != nil {
		t.Fatalf("failed to encode packet: %v", err)
	}
	packet[len(packet)-1] ^= 0x01

	if _, err := dec.DecodePacket(packet); err == nil {
		t.Fatal("expected tampered packet to fail")
	}
}

func TestUDPFramerPacketLengthNormalization(t *testing.T) {
	psk := bytes.Repeat([]byte{0x94}, 32)
	clientRandom := []byte("client-random-frame-udp-size")
	serverRandom := []byte("server-random-frame-udp-size")
	encKey, authKey := otcrypto.DeriveSessionKeys(psk, clientRandom, serverRandom)
	keys := testDirectionalKeys(encKey, authKey)

	enc, _ := NewUDPFramer(psk, keys.c2sEnc, keys.c2sAuth, keys.s2cEnc, keys.s2cAuth)

	tests := []struct {
		payloadLen int
		wantLen    int
	}{
		{payloadLen: 0, wantLen: udpTargetSmall},
		{payloadLen: 600, wantLen: udpTargetMedium},
		{payloadLen: 1400, wantLen: udpTargetLarge},
	}

	for _, tc := range tests {
		payload := bytes.Repeat([]byte{0x5a}, tc.payloadLen)
		packet, err := enc.EncodePacket(payload)
		if err != nil {
			t.Fatalf("failed to encode payload len %d: %v", tc.payloadLen, err)
		}
		if len(packet) != tc.wantLen {
			t.Fatalf("unexpected normalized length for payload %d: got %d want %d", tc.payloadLen, len(packet), tc.wantLen)
		}
	}
}
