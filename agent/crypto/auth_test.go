package crypto

import (
	"bytes"
	"testing"
)

func TestComputeMACVerifyRoundTrip(t *testing.T) {
	psk := bytes.Repeat([]byte{0x71}, 32)
	clientRandom := []byte("client-random-auth-roundtrip")
	serverRandom := []byte("server-random-auth-roundtrip")
	_, authKey := DeriveSessionKeys(psk, clientRandom, serverRandom)

	ciphertext := bytes.Repeat([]byte{0xAB, 0xCD, 0xEF, 0x01}, 32)
	mac, err := ComputeMAC(authKey, 42, ciphertext)
	if err != nil {
		t.Fatalf("failed to compute mac: %v", err)
	}

	if !VerifyMAC(authKey, 42, ciphertext, mac) {
		t.Fatal("expected MAC verification to succeed")
	}
}

func TestVerifyMACRejectsTamperedCiphertext(t *testing.T) {
	psk := bytes.Repeat([]byte{0x72}, 32)
	clientRandom := []byte("client-random-auth-ciphertext")
	serverRandom := []byte("server-random-auth-ciphertext")
	_, authKey := DeriveSessionKeys(psk, clientRandom, serverRandom)

	ciphertext := []byte("sealed-payload")
	mac, err := ComputeMAC(authKey, 7, ciphertext)
	if err != nil {
		t.Fatalf("failed to compute mac: %v", err)
	}

	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 0x01
	if VerifyMAC(authKey, 7, tampered, mac) {
		t.Fatal("verification unexpectedly succeeded for tampered ciphertext")
	}
}

func TestVerifyMACRejectsTamperedMAC(t *testing.T) {
	psk := bytes.Repeat([]byte{0x73}, 32)
	clientRandom := []byte("client-random-auth-mac")
	serverRandom := []byte("server-random-auth-mac")
	_, authKey := DeriveSessionKeys(psk, clientRandom, serverRandom)

	ciphertext := []byte("authenticated-ciphertext")
	mac, err := ComputeMAC(authKey, 9, ciphertext)
	if err != nil {
		t.Fatalf("failed to compute mac: %v", err)
	}

	mac[0] ^= 0x01
	if VerifyMAC(authKey, 9, ciphertext, mac) {
		t.Fatal("verification unexpectedly succeeded for tampered MAC")
	}
}

func TestComputeMACDifferentCountersProduceDifferentMACs(t *testing.T) {
	psk := bytes.Repeat([]byte{0x74}, 32)
	clientRandom := []byte("client-random-auth-counter")
	serverRandom := []byte("server-random-auth-counter")
	_, authKey := DeriveSessionKeys(psk, clientRandom, serverRandom)

	ciphertext := []byte("counter-sensitive-ciphertext")
	macA, err := ComputeMAC(authKey, 100, ciphertext)
	if err != nil {
		t.Fatalf("failed to compute mac A: %v", err)
	}
	macB, err := ComputeMAC(authKey, 101, ciphertext)
	if err != nil {
		t.Fatalf("failed to compute mac B: %v", err)
	}

	if macA == macB {
		t.Fatal("different counters produced identical MACs")
	}
}
