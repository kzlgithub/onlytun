package crypto

import (
	"bytes"
	"testing"
)

func TestDeriveSessionKeysDeterministic(t *testing.T) {
	psk := bytes.Repeat([]byte{0x11}, 32)
	clientRandom := []byte("client-random-001")
	serverRandom := []byte("server-random-001")

	encA, authA := DeriveSessionKeys(psk, clientRandom, serverRandom)
	encB, authB := DeriveSessionKeys(psk, clientRandom, serverRandom)

	if !bytes.Equal(encA, encB) {
		t.Fatal("encryption key derivation is not deterministic")
	}
	if !bytes.Equal(authA, authB) {
		t.Fatal("authentication key derivation is not deterministic")
	}
	if len(encA) != 32 || len(authA) != 32 {
		t.Fatalf("unexpected session key lengths: enc=%d auth=%d", len(encA), len(authA))
	}
}

func TestDifferentLabelsProduceDifferentOutputs(t *testing.T) {
	psk := bytes.Repeat([]byte{0x22}, 32)
	clientRandom := []byte("client-random-002")
	serverRandom := []byte("server-random-002")

	encKey, authKey := DeriveSessionKeys(psk, clientRandom, serverRandom)
	if bytes.Equal(encKey, authKey) {
		t.Fatal("different session key labels produced the same output")
	}

	hashEnc := deriveHash(labelEnc, psk, clientRandom, serverRandom)
	hashAuth := deriveHash(labelAuth, psk, clientRandom, serverRandom)
	hashConst := deriveHash(labelConst, psk)
	hashRot := deriveHash(labelRot, psk)
	hashSBox := deriveHash(labelSBox, psk)
	hashBitRot := deriveHash(labelBitRot, psk)

	hashes := map[string][32]byte{
		labelEnc:    hashEnc,
		labelAuth:   hashAuth,
		labelConst:  hashConst,
		labelRot:    hashRot,
		labelSBox:   hashSBox,
		labelBitRot: hashBitRot,
	}

	for labelA, hashA := range hashes {
		for labelB, hashB := range hashes {
			if labelA == labelB {
				continue
			}
			if hashA == hashB {
				t.Fatalf("labels %q and %q produced the same output", labelA, labelB)
			}
		}
	}
}

func TestDifferentPSKProducesDifferentOutputs(t *testing.T) {
	pskA := bytes.Repeat([]byte{0x33}, 32)
	pskB := bytes.Repeat([]byte{0x44}, 32)
	clientRandom := []byte("client-random-003")
	serverRandom := []byte("server-random-003")

	encA, authA := DeriveSessionKeys(pskA, clientRandom, serverRandom)
	encB, authB := DeriveSessionKeys(pskB, clientRandom, serverRandom)
	if bytes.Equal(encA, encB) {
		t.Fatal("different PSKs produced the same encryption key")
	}
	if bytes.Equal(authA, authB) {
		t.Fatal("different PSKs produced the same authentication key")
	}

	if DeriveCipherConstants(pskA) == DeriveCipherConstants(pskB) {
		t.Fatal("different PSKs produced the same cipher constants")
	}
	if DeriveRotationTable(pskA) == DeriveRotationTable(pskB) {
		t.Fatal("different PSKs produced the same rotation table")
	}
	if DeriveSBoxSeed(pskA) == DeriveSBoxSeed(pskB) {
		t.Fatal("different PSKs produced the same s-box seed")
	}
	if DeriveBitRotBase(pskA) == DeriveBitRotBase(pskB) {
		t.Fatal("different PSKs produced the same bit rotation base")
	}
}

func TestDeriveCipherConstantsDeterministic(t *testing.T) {
	psk := bytes.Repeat([]byte{0x55}, 32)

	a := DeriveCipherConstants(psk)
	b := DeriveCipherConstants(psk)

	if a != b {
		t.Fatal("cipher constants derivation is not deterministic")
	}
}

func TestDeriveRotationTableDeterministicAndInRange(t *testing.T) {
	psk := bytes.Repeat([]byte{0x66}, 32)

	a := DeriveRotationTable(psk)
	b := DeriveRotationTable(psk)

	if a != b {
		t.Fatal("rotation table derivation is not deterministic")
	}
	for i, value := range a {
		if value < 5 || value > 27 {
			t.Fatalf("rotation value at index %d out of range: %d", i, value)
		}
	}
}

func TestDeriveSBoxSeedDeterministic(t *testing.T) {
	psk := bytes.Repeat([]byte{0x77}, 32)

	a := DeriveSBoxSeed(psk)
	b := DeriveSBoxSeed(psk)

	if a != b {
		t.Fatal("s-box seed derivation is not deterministic")
	}
}

func TestDeriveBitRotBaseDeterministicAndInRange(t *testing.T) {
	psk := bytes.Repeat([]byte{0x88}, 32)

	a := DeriveBitRotBase(psk)
	b := DeriveBitRotBase(psk)

	if a != b {
		t.Fatal("bit rotation base derivation is not deterministic")
	}
	if a > 7 {
		t.Fatalf("bit rotation base out of range: %d", a)
	}
}
