package crypto

import (
	"encoding/binary"
	"errors"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/poly1305"
)

const (
	authKeySize = 32
	// AuthTagSize is the size of the Poly1305 authentication tag.
	AuthTagSize = 16
	macLabel    = "OnlyTun-mac-v1"
)

// ComputeMAC computes a Poly1305 MAC for a packet ciphertext.
// authKey must be the 32-byte session authentication key.
func ComputeMAC(authKey []byte, packetCounter uint64, ciphertext []byte) ([16]byte, error) {
	var out [16]byte

	oneTimeKey, err := derivePacketMACKey(authKey, packetCounter)
	if err != nil {
		return out, err
	}

	poly1305.Sum(&out, ciphertext, &oneTimeKey)
	return out, nil
}

// VerifyMAC verifies a packet MAC without revealing specific failure reasons.
func VerifyMAC(authKey []byte, packetCounter uint64, ciphertext []byte, mac [16]byte) bool {
	oneTimeKey, err := derivePacketMACKey(authKey, packetCounter)
	if err != nil {
		return false
	}

	return poly1305.Verify(&mac, ciphertext, &oneTimeKey)
}

func derivePacketMACKey(authKey []byte, packetCounter uint64) ([32]byte, error) {
	var out [32]byte
	if len(authKey) != authKeySize {
		return out, errors.New("crypto: invalid auth key length")
	}

	h, err := blake2b.New256(nil)
	if err != nil {
		return out, err
	}

	var counter [8]byte
	binary.LittleEndian.PutUint64(counter[:], packetCounter)

	_, _ = h.Write(authKey)
	_, _ = h.Write([]byte(macLabel))
	_, _ = h.Write(counter[:])

	copy(out[:], h.Sum(nil))
	return out, nil
}
