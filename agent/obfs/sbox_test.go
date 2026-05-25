package obfs

import (
	"bytes"
	"testing"
)

func TestSBoxRoundTrip(t *testing.T) {
	psk := bytes.Repeat([]byte{0x31}, 32)
	sbox := NewSBox(psk)

	original := bytes.Repeat([]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77}, 16)
	data := append([]byte(nil), original...)

	sbox.Obfuscate(data)
	sbox.Deobfuscate(data)

	if !bytes.Equal(data, original) {
		t.Fatal("s-box deobfuscation did not restore original data")
	}
}

func TestSBoxDifferentPSKProducesDifferentOutput(t *testing.T) {
	pskA := bytes.Repeat([]byte{0x32}, 32)
	pskB := bytes.Repeat([]byte{0x33}, 32)

	sboxA := NewSBox(pskA)
	sboxB := NewSBox(pskB)

	dataA := bytes.Repeat([]byte{0x7a, 0x42, 0x19, 0xef}, 16)
	dataB := append([]byte(nil), dataA...)

	sboxA.Obfuscate(dataA)
	sboxB.Obfuscate(dataB)

	if bytes.Equal(dataA, dataB) {
		t.Fatal("different PSKs produced identical s-box obfuscation")
	}
}
