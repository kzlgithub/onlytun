package obfs

import (
	"bytes"
	"testing"
)

func TestBitRotRoundTrip(t *testing.T) {
	psk := bytes.Repeat([]byte{0x41}, 32)
	obfs := NewBitRotObfs(psk)

	original := bytes.Repeat([]byte{0x0f, 0xf0, 0x55, 0xaa, 0x99}, 20)
	data := append([]byte(nil), original...)

	obfs.Obfuscate(data, 17)
	obfs.Deobfuscate(data, 17)

	if !bytes.Equal(data, original) {
		t.Fatal("bit rotation deobfuscation did not restore original data")
	}
}

func TestBitRotDifferentPSKProducesDifferentOutput(t *testing.T) {
	pskA := bytes.Repeat([]byte{0x42}, 32)
	pskB := bytes.Repeat([]byte{0x43}, 32)

	obfsA := NewBitRotObfs(pskA)
	obfsB := NewBitRotObfs(pskB)

	dataA := bytes.Repeat([]byte{0x3c, 0xc3, 0x5a}, 24)
	dataB := append([]byte(nil), dataA...)

	obfsA.Obfuscate(dataA, 5)
	obfsB.Obfuscate(dataB, 5)

	if bytes.Equal(dataA, dataB) {
		t.Fatal("different PSKs produced identical bit rotation output")
	}
}

func TestBitRotDifferentCountersProduceDifferentOutput(t *testing.T) {
	psk := bytes.Repeat([]byte{0x44}, 32)
	obfs := NewBitRotObfs(psk)

	dataA := bytes.Repeat([]byte{0x96, 0x69, 0xa5, 0x5a}, 16)
	dataB := append([]byte(nil), dataA...)

	obfs.Obfuscate(dataA, 1)
	obfs.Obfuscate(dataB, 2)

	if bytes.Equal(dataA, dataB) {
		t.Fatal("different counters produced identical bit rotation output")
	}
}
