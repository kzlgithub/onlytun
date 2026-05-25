package crypto

import (
	"bytes"
	"math"
	"testing"
)

func TestPrivateCipherEncryptDecryptRoundTrip(t *testing.T) {
	psk := bytes.Repeat([]byte{0x10}, 32)
	clientRandom := []byte("client-random-roundtrip")
	serverRandom := []byte("server-random-roundtrip")
	nonce := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

	key, _ := DeriveSessionKeys(psk, clientRandom, serverRandom)
	cipherEnc, err := NewPrivateCipher(psk, key, nonce)
	if err != nil {
		t.Fatalf("failed to create encrypt cipher: %v", err)
	}

	plaintext := bytes.Repeat([]byte("OnlyTun-private-stream-cipher-"), 8)
	ciphertext := make([]byte, len(plaintext))
	cipherEnc.XORKeyStream(ciphertext, plaintext)

	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext unexpectedly matches plaintext")
	}

	cipherDec, err := NewPrivateCipher(psk, key, nonce)
	if err != nil {
		t.Fatalf("failed to create decrypt cipher: %v", err)
	}

	decrypted := make([]byte, len(ciphertext))
	cipherDec.XORKeyStream(decrypted, ciphertext)

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatal("decrypted plaintext does not match original plaintext")
	}
}

func TestPrivateCipherDifferentPSKProducesDifferentCiphertext(t *testing.T) {
	pskA := bytes.Repeat([]byte{0x21}, 32)
	pskB := bytes.Repeat([]byte{0x22}, 32)
	clientRandom := []byte("client-random-different-psk")
	serverRandom := []byte("server-random-different-psk")
	nonce := []byte{11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0}
	plaintext := bytes.Repeat([]byte("OnlyTun-private-stream-cipher-"), 4)

	keyA, _ := DeriveSessionKeys(pskA, clientRandom, serverRandom)
	keyB, _ := DeriveSessionKeys(pskB, clientRandom, serverRandom)

	cipherA, err := NewPrivateCipher(pskA, keyA, nonce)
	if err != nil {
		t.Fatalf("failed to create cipher A: %v", err)
	}
	cipherB, err := NewPrivateCipher(pskB, keyB, nonce)
	if err != nil {
		t.Fatalf("failed to create cipher B: %v", err)
	}

	ciphertextA := make([]byte, len(plaintext))
	ciphertextB := make([]byte, len(plaintext))
	cipherA.XORKeyStream(ciphertextA, plaintext)
	cipherB.XORKeyStream(ciphertextB, plaintext)

	if bytes.Equal(ciphertextA, ciphertextB) {
		t.Fatal("different PSKs produced identical ciphertext")
	}
}

func TestPrivateCipherEmptyData(t *testing.T) {
	psk := bytes.Repeat([]byte{0x30}, 32)
	clientRandom := []byte("client-random-empty")
	serverRandom := []byte("server-random-empty")
	nonce := []byte{1, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144}

	key, _ := DeriveSessionKeys(psk, clientRandom, serverRandom)
	c, err := NewPrivateCipher(psk, key, nonce)
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	var dst []byte
	var src []byte
	c.XORKeyStream(dst, src)

	if len(dst) != 0 {
		t.Fatalf("expected empty output, got %d bytes", len(dst))
	}
}

func TestPrivateCipherXORKeyStreamInPlace(t *testing.T) {
	psk := bytes.Repeat([]byte{0x51}, 32)
	clientRandom := []byte("client-random-inplace")
	serverRandom := []byte("server-random-inplace")
	nonce := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}

	key, _ := DeriveSessionKeys(psk, clientRandom, serverRandom)
	plain := bytes.Repeat([]byte("OnlyTun-in-place-"), 10)
	buf := append([]byte(nil), plain...)

	enc, err := NewPrivateCipher(psk, key, nonce)
	if err != nil {
		t.Fatalf("failed to create encrypt cipher: %v", err)
	}
	enc.XORKeyStream(buf, buf)

	dec, err := NewPrivateCipher(psk, key, nonce)
	if err != nil {
		t.Fatalf("failed to create decrypt cipher: %v", err)
	}
	dec.XORKeyStream(buf, buf)

	if !bytes.Equal(buf, plain) {
		t.Fatal("in-place XORKeyStream failed to round-trip")
	}
}

func TestPrivateCipherResetReusesInitialState(t *testing.T) {
	psk := bytes.Repeat([]byte{0x40}, 32)
	clientRandom := []byte("client-random-reset")
	serverRandom := []byte("server-random-reset")
	nonce := []byte{9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 1, 2}
	plaintext := bytes.Repeat([]byte("reset-check-"), 16)

	key, _ := DeriveSessionKeys(psk, clientRandom, serverRandom)
	c, err := NewPrivateCipher(psk, key, nonce)
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	first := make([]byte, len(plaintext))
	second := make([]byte, len(plaintext))

	c.XORKeyStream(first, plaintext)
	c.Reset()
	c.XORKeyStream(second, plaintext)

	if !bytes.Equal(first, second) {
		t.Fatal("reset did not restore the initial keystream")
	}
}

func TestPrivateCipherPanicsWhenKeystreamExhausted(t *testing.T) {
	psk := bytes.Repeat([]byte{0x61}, 32)
	clientRandom := []byte("client-random-exhaust")
	serverRandom := []byte("server-random-exhaust")
	nonce := []byte{2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37}

	key, _ := DeriveSessionKeys(psk, clientRandom, serverRandom)
	c, err := NewPrivateCipher(psk, key, nonce)
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	c.initialState[12] = math.MaxUint32
	c.Reset()

	firstBlock := make([]byte, blockSize)
	secondBlock := make([]byte, blockSize)

	c.XORKeyStream(firstBlock, firstBlock)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when keystream is exhausted")
		}
	}()

	c.XORKeyStream(secondBlock, secondBlock)
}
