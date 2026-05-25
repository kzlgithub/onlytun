package frame

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"

	otcrypto "github.com/onlytun/agent/crypto"
	"github.com/onlytun/agent/obfs"
)

const (
	udpNonceSize    = 8
	udpCounterSize  = 8
	udpLengthSize   = 2
	udpHeaderSize   = udpNonceSize + udpCounterSize
	udpMACSize      = 16
	udpTargetSmall  = 512
	udpTargetMedium = 1024
	udpTargetLarge  = 1500
	udpReplayWindow = 128
)

var (
	errInvalidPacketSize = errors.New("frame: invalid packet size")
	errReplayDetected    = errors.New("frame: replay detected")
	errPacketTooLarge    = errors.New("frame: packet too large")
)

// UDP frame format on the wire:
// [Nonce 8B plaintext][Counter 8B plaintext][encrypted body: PayloadLen 2B + Payload + Padding][MAC 16B]
// S-Box and BitRot obfuscation apply only to the encrypted body, not to the plaintext header or MAC.
type UDPFramer struct {
	psk          []byte
	writeEncKey  []byte
	writeAuthKey []byte
	readEncKey   []byte
	readAuthKey  []byte
	sbox         *obfs.SBox
	bitrot       *obfs.BitRotObfs

	writeCounter uint64
	replay       replayWindow
}

// NewUDPFramer creates a UDP framer with isolated write/read keys.
func NewUDPFramer(psk, writeEncKey, writeAuthKey, readEncKey, readAuthKey []byte) (*UDPFramer, error) {
	if len(psk) != 32 {
		return nil, errors.New("frame: invalid psk length")
	}
	if len(writeEncKey) != 32 || len(readEncKey) != 32 {
		return nil, errors.New("frame: invalid encryption key length")
	}
	if len(writeAuthKey) != 32 || len(readAuthKey) != 32 {
		return nil, errors.New("frame: invalid auth key length")
	}

	return &UDPFramer{
		psk:          append([]byte(nil), psk...),
		writeEncKey:  append([]byte(nil), writeEncKey...),
		writeAuthKey: append([]byte(nil), writeAuthKey...),
		readEncKey:   append([]byte(nil), readEncKey...),
		readAuthKey:  append([]byte(nil), readAuthKey...),
		sbox:         obfs.NewSBox(psk),
		bitrot:       obfs.NewBitRotObfs(psk),
	}, nil
}

// EncodePacket keeps the 16-byte nonce/counter header in plaintext and encrypts/obfuscates only the body.
func (f *UDPFramer) EncodePacket(payload []byte) ([]byte, error) {
	bodyLen, packetLen, err := normalizedPacketSizes(len(payload))
	if err != nil {
		return nil, err
	}

	counter := f.writeCounter
	var nonce [udpNonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("frame: random nonce: %w", err)
	}

	header := make([]byte, udpHeaderSize)
	copy(header[:udpNonceSize], nonce[:])
	binary.BigEndian.PutUint64(header[udpNonceSize:udpHeaderSize], counter)

	plain := make([]byte, bodyLen-udpHeaderSize)
	binary.BigEndian.PutUint16(plain[:udpLengthSize], uint16(len(payload)))
	copy(plain[udpLengthSize:], payload)
	if _, err := rand.Read(plain[udpLengthSize+len(payload):]); err != nil {
		return nil, fmt.Errorf("frame: random padding: %w", err)
	}

	cipherNonce := buildUDPCipherNonce(nonce)
	cipher, err := otcrypto.NewPrivateCipher(f.psk, f.writeEncKey, cipherNonce[:])
	if err != nil {
		return nil, fmt.Errorf("frame: create cipher: %w", err)
	}

	ciphertext := make([]byte, len(plain))
	cipher.XORKeyStream(ciphertext, plain)

	macInput := make([]byte, len(header)+len(ciphertext))
	copy(macInput, header)
	copy(macInput[len(header):], ciphertext)

	mac, err := otcrypto.ComputeMAC(f.writeAuthKey, counter, macInput)
	if err != nil {
		return nil, fmt.Errorf("frame: compute mac: %w", err)
	}

	// Only the ciphertext body is obfuscated; the header and MAC stay parseable.
	body := append([]byte(nil), ciphertext...)
	f.sbox.Obfuscate(body)
	f.bitrot.Obfuscate(body, counter)

	packet := make([]byte, packetLen)
	copy(packet[:udpHeaderSize], header)
	copy(packet[udpHeaderSize:bodyLen], body)
	copy(packet[bodyLen:], mac[:])

	f.writeCounter++
	return packet, nil
}

// DecodePacket reads the plaintext header first, then deobfuscates/decrypts only the body, and finally verifies MAC.
func (f *UDPFramer) DecodePacket(data []byte) ([]byte, error) {
	bodyLen, err := expectedUDPBodyLength(len(data))
	if err != nil {
		return nil, err
	}

	header := append([]byte(nil), data[:udpHeaderSize]...)
	counter := binary.BigEndian.Uint64(header[udpNonceSize:udpHeaderSize])
	body := append([]byte(nil), data[udpHeaderSize:bodyLen]...)
	var mac [udpMACSize]byte
	copy(mac[:], data[bodyLen:])

	ciphertext := append([]byte(nil), body...)
	f.bitrot.Deobfuscate(ciphertext, counter)
	f.sbox.Deobfuscate(ciphertext)

	macInput := make([]byte, len(header)+len(ciphertext))
	copy(macInput, header)
	copy(macInput[len(header):], ciphertext)
	if !otcrypto.VerifyMAC(f.readAuthKey, counter, macInput, mac) {
		return nil, errAuthFailed
	}
	if err := f.replay.accept(counter); err != nil {
		return nil, err
	}

	plain, err := f.decodeBody(header[:udpNonceSize], ciphertext)
	if err != nil {
		return nil, err
	}

	payloadLen := int(binary.BigEndian.Uint16(plain[:udpLengthSize]))
	if payloadLen > len(plain)-udpLengthSize {
		return nil, errInvalidPacketSize
	}

	return append([]byte(nil), plain[udpLengthSize:udpLengthSize+payloadLen]...), nil
}

func (f *UDPFramer) decodeBody(nonce []byte, ciphertext []byte) ([]byte, error) {
	var nonceArr [udpNonceSize]byte
	copy(nonceArr[:], nonce)

	cipherNonce := buildUDPCipherNonce(nonceArr)
	cipher, err := otcrypto.NewPrivateCipher(f.psk, f.readEncKey, cipherNonce[:])
	if err != nil {
		return nil, fmt.Errorf("frame: create cipher: %w", err)
	}

	plain := make([]byte, len(ciphertext))
	cipher.XORKeyStream(plain, ciphertext)
	return plain, nil
}

func normalizedPacketSizes(payloadLen int) (bodyLen, packetLen int, err error) {
	actual := udpHeaderSize + udpLengthSize + payloadLen
	switch {
	case actual+udpMACSize <= udpTargetSmall:
		return udpTargetSmall - udpMACSize, udpTargetSmall, nil
	case actual+udpMACSize <= udpTargetMedium:
		return udpTargetMedium - udpMACSize, udpTargetMedium, nil
	case actual+udpMACSize <= udpTargetLarge:
		return udpTargetLarge - udpMACSize, udpTargetLarge, nil
	default:
		return 0, 0, errPacketTooLarge
	}
}

func expectedUDPBodyLength(packetLen int) (int, error) {
	switch packetLen {
	case udpTargetSmall, udpTargetMedium, udpTargetLarge:
		return packetLen - udpMACSize, nil
	default:
		return 0, errInvalidPacketSize
	}
}

func buildUDPCipherNonce(nonce [udpNonceSize]byte) [12]byte {
	var out [12]byte
	copy(out[:udpNonceSize], nonce[:])
	return out
}

type replayWindow struct {
	initialized bool
	maxCounter  uint64
	bits        [2]uint64
}

func (w *replayWindow) accept(counter uint64) error {
	if !w.initialized {
		w.initialized = true
		w.maxCounter = counter
		w.bits[0] = 1
		return nil
	}

	if counter > w.maxCounter {
		shift := counter - w.maxCounter
		w.shift(shift)
		w.maxCounter = counter
		w.bits[0] |= 1
		return nil
	}

	distance := w.maxCounter - counter
	if distance >= udpReplayWindow {
		return errReplayDetected
	}
	if w.has(distance) {
		return errReplayDetected
	}
	w.set(distance)
	return nil
}

func (w *replayWindow) shift(n uint64) {
	if n >= udpReplayWindow {
		w.bits = [2]uint64{}
		return
	}
	if n >= 64 {
		w.bits[1] = w.bits[0] << (n - 64)
		w.bits[0] = 0
	} else if n > 0 {
		w.bits[1] = (w.bits[1] << n) | (w.bits[0] >> (64 - n))
		w.bits[0] <<= n
	}
}

func (w *replayWindow) has(distance uint64) bool {
	if distance < 64 {
		return (w.bits[0] & (uint64(1) << distance)) != 0
	}
	return (w.bits[1] & (uint64(1) << (distance - 64))) != 0
}

func (w *replayWindow) set(distance uint64) {
	if distance < 64 {
		w.bits[0] |= uint64(1) << distance
		return
	}
	w.bits[1] |= uint64(1) << (distance - 64)
}
