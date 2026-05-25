package session

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	otcrypto "github.com/onlytun/agent/crypto"
	"golang.org/x/crypto/blake2b"
)

const (
	clientHelloSize   = 76 // 12B random nonce + 64B ciphertext
	serverHelloSize   = 48
	clientConfirmSize = 4
	handshakePSKSize  = 32
	handshakeTimeout  = 5 * time.Second
	allowedTimeSkew   = 60 * time.Second
	handshakeKeyLabel = "OnlyTun-handshake-v1"
	step2KeyLabel     = "OnlyTun-handshake-step2-key-v1"
	step2NonceLabel   = "OnlyTun-handshake-step2-nonce-v1"
	step3NonceLabel   = "OnlyTun-handshake-step3-nonce-v1"
	sessionNonceLabel = "OnlyTun-session-nonce-v1"
	c2sEncKeyLabel    = "OnlyTun-c2s-enc-v1"
	c2sAuthKeyLabel   = "OnlyTun-c2s-auth-v1"
	s2cEncKeyLabel    = "OnlyTun-s2c-enc-v1"
	s2cAuthKeyLabel   = "OnlyTun-s2c-auth-v1"
)

var (
	errInvalidPSK         = errors.New("session: invalid psk length")
	errInvalidTimestamp   = errors.New("session: invalid timestamp")
	errInvalidServerProof = errors.New("session: invalid server proof")
	errInvalidClientProof = errors.New("session: invalid client confirmation")
	nowFunc               = time.Now
)

// HandshakeResult carries the session keys needed by later tunnel layers.
type HandshakeResult struct {
	EncKey     []byte
	AuthKey    []byte
	Nonce      []byte
	C2SEncKey  []byte
	C2SAuthKey []byte
	S2CEncKey  []byte
	S2CAuthKey []byte
}

// ClientHandshake performs the client side of the PSK handshake.
func ClientHandshake(conn net.Conn, psk []byte) (*HandshakeResult, error) {
	if len(psk) != handshakePSKSize {
		return nil, errInvalidPSK
	}
	if err := conn.SetDeadline(time.Now().Add(handshakeTimeout)); err == nil {
		defer conn.SetDeadline(time.Time{})
	}

	clientRandom := make([]byte, 32)
	padding := make([]byte, 24)
	if _, err := rand.Read(clientRandom); err != nil {
		return nil, fmt.Errorf("session: generate client random: %w", err)
	}
	if _, err := rand.Read(padding); err != nil {
		return nil, fmt.Errorf("session: generate client padding: %w", err)
	}

	clientHello, err := buildClientHelloPacket(psk, clientRandom, uint64(nowFunc().Unix()), padding)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(clientHello); err != nil {
		return nil, err
	}

	serverHello := make([]byte, serverHelloSize)
	if _, err := io.ReadFull(conn, serverHello); err != nil {
		return nil, err
	}

	encKey, authKey, serverRandom, proof, err := decryptServerHello(psk, clientRandom, serverHello)
	if err != nil {
		return nil, err
	}

	expectedProof, err := computeHandshakeProof(authKey, clientRandom, serverRandom)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(proof, expectedProof[:]) {
		return nil, errInvalidServerProof
	}

	confirm, err := encryptWithKey(psk, encKey, deriveNonce(psk, step3NonceLabel, clientRandom, serverRandom), expectedProof[:clientConfirmSize])
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(confirm); err != nil {
		return nil, err
	}

	return buildHandshakeResult(psk, encKey, authKey, clientRandom, serverRandom), nil
}

// ServerHandshake performs the server side of the PSK handshake.
func ServerHandshake(conn net.Conn, psk []byte) (*HandshakeResult, error) {
	if len(psk) != handshakePSKSize {
		conn.Close()
		return nil, errInvalidPSK
	}
	if err := conn.SetDeadline(time.Now().Add(handshakeTimeout)); err == nil {
		defer conn.SetDeadline(time.Time{})
	}

	clientHello := make([]byte, clientHelloSize)
	if _, err := io.ReadFull(conn, clientHello); err != nil {
		conn.Close()
		return nil, err
	}

	clientRandom, timestamp, err := parseClientHello(psk, clientHello)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !timestampWithinWindow(timestamp, nowFunc()) {
		conn.Close()
		return nil, errInvalidTimestamp
	}
	if globalReplayCache.CheckAndAdd(clientRandom, nowFunc()) {
		conn.Close()
		return nil, errors.New("session: replayed handshake")
	}

	serverRandom := make([]byte, 32)
	if _, err := rand.Read(serverRandom); err != nil {
		conn.Close()
		return nil, fmt.Errorf("session: generate server random: %w", err)
	}

	encKey, authKey := otcrypto.DeriveSessionKeys(psk, clientRandom, serverRandom)
	proof, err := computeHandshakeProof(authKey, clientRandom, serverRandom)
	if err != nil {
		conn.Close()
		return nil, err
	}

	serverHelloPlain := make([]byte, serverHelloSize)
	copy(serverHelloPlain[:32], serverRandom)
	copy(serverHelloPlain[32:], proof[:])

	serverHello, err := encryptWithKey(psk, deriveStep2TransportKey(psk, clientRandom), deriveNonce(psk, step2NonceLabel, clientRandom), serverHelloPlain)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := conn.Write(serverHello); err != nil {
		conn.Close()
		return nil, err
	}

	confirm := make([]byte, clientConfirmSize)
	if _, err := io.ReadFull(conn, confirm); err != nil {
		conn.Close()
		return nil, err
	}

	confirmPlain, err := decryptWithKey(psk, encKey, deriveNonce(psk, step3NonceLabel, clientRandom, serverRandom), confirm)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !bytes.Equal(confirmPlain, proof[:clientConfirmSize]) {
		conn.Close()
		return nil, errInvalidClientProof
	}

	return buildHandshakeResult(psk, encKey, authKey, clientRandom, serverRandom), nil
}

// buildClientHelloPacket serializes clientRandom(32B) + timestamp(8B) + padding(24B),
// encrypts the 64-byte plaintext, and prefixes it with a 12-byte random nonce.
func buildClientHelloPacket(psk, clientRandom []byte, timestamp uint64, padding []byte) ([]byte, error) {
	if len(clientRandom) != 32 {
		return nil, errors.New("session: invalid client random length")
	}
	if len(padding) != 24 {
		return nil, errors.New("session: invalid padding length")
	}

	plain := make([]byte, 64)
	copy(plain[:32], clientRandom)
	binary.BigEndian.PutUint64(plain[32:40], timestamp)
	copy(plain[40:], padding)

	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("session: generate step1 nonce: %w", err)
	}

	ciphertext, err := encryptWithKey(psk, deriveHandshakeKey(psk), nonce, plain)
	if err != nil {
		return nil, err
	}

	packet := make([]byte, clientHelloSize)
	copy(packet[:12], nonce)
	copy(packet[12:], ciphertext)
	return packet, nil
}

// parseClientHello decrypts a client hello whose plaintext layout is
// clientRandom(32B) + timestamp(8B) + padding(24B).
func parseClientHello(psk, packet []byte) ([]byte, uint64, error) {
	if len(packet) < 12 {
		return nil, 0, errors.New("session: client hello too short")
	}
	nonce := packet[:12]
	plain, err := decryptWithKey(psk, deriveHandshakeKey(psk), nonce, packet[12:])
	if err != nil {
		return nil, 0, err
	}

	clientRandom := append([]byte(nil), plain[:32]...)
	timestamp := binary.BigEndian.Uint64(plain[32:40])
	return clientRandom, timestamp, nil
}

func decryptServerHello(psk, clientRandom, packet []byte) ([]byte, []byte, []byte, []byte, error) {
	transportKey := deriveStep2TransportKey(psk, clientRandom)
	plain, err := decryptWithKey(psk, transportKey, deriveNonce(psk, step2NonceLabel, clientRandom), packet)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	serverRandom := append([]byte(nil), plain[:32]...)
	encKey, authKey := otcrypto.DeriveSessionKeys(psk, clientRandom, serverRandom)
	return encKey, authKey, serverRandom, append([]byte(nil), plain[32:]...), nil
}

func deriveHandshakeKey(psk []byte) []byte {
	sum := hashParts(psk, []byte(handshakeKeyLabel))
	return sum[:]
}

func deriveStep2TransportKey(psk, clientRandom []byte) []byte {
	sum := hashParts(psk, []byte(step2KeyLabel), clientRandom)
	return sum[:]
}

func deriveNonce(psk []byte, label string, parts ...[]byte) []byte {
	args := make([][]byte, 0, len(parts)+2)
	args = append(args, psk, []byte(label))
	args = append(args, parts...)
	sum := hashParts(args...)
	return append([]byte(nil), sum[:12]...)
}

func encryptWithKey(psk, key, nonce, plain []byte) ([]byte, error) {
	cipher, err := otcrypto.NewPrivateCipher(psk, key, nonce)
	if err != nil {
		return nil, err
	}

	out := make([]byte, len(plain))
	cipher.XORKeyStream(out, plain)
	return out, nil
}

func decryptWithKey(psk, key, nonce, ciphertext []byte) ([]byte, error) {
	return encryptWithKey(psk, key, nonce, ciphertext)
}

func computeHandshakeProof(authKey, clientRandom, serverRandom []byte) ([16]byte, error) {
	transcript := make([]byte, 0, len(clientRandom)+len(serverRandom))
	transcript = append(transcript, clientRandom...)
	transcript = append(transcript, serverRandom...)
	return otcrypto.ComputeMAC(authKey, 0, transcript)
}

func buildHandshakeResult(psk, encKey, authKey, clientRandom, serverRandom []byte) *HandshakeResult {
	return &HandshakeResult{
		EncKey:     append([]byte(nil), encKey...),
		AuthKey:    append([]byte(nil), authKey...),
		Nonce:      deriveNonce(psk, sessionNonceLabel, clientRandom, serverRandom),
		C2SEncKey:  deriveDirectionalKey(encKey, c2sEncKeyLabel),
		C2SAuthKey: deriveDirectionalKey(authKey, c2sAuthKeyLabel),
		S2CEncKey:  deriveDirectionalKey(encKey, s2cEncKeyLabel),
		S2CAuthKey: deriveDirectionalKey(authKey, s2cAuthKeyLabel),
	}
}

func deriveDirectionalKey(rootKey []byte, label string) []byte {
	sum := hashParts(rootKey, []byte(label))
	return append([]byte(nil), sum[:]...)
}

func timestampWithinWindow(ts uint64, now time.Time) bool {
	nowUnix := now.Unix()
	if ts > uint64(nowUnix+int64(allowedTimeSkew/time.Second)) {
		return false
	}
	if nowUnix > int64(ts)+int64(allowedTimeSkew/time.Second) {
		return false
	}
	return true
}

func hashParts(parts ...[]byte) [32]byte {
	h, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	for _, part := range parts {
		var size [4]byte
		binary.LittleEndian.PutUint32(size[:], uint32(len(part)))
		_, _ = h.Write(size[:])
		_, _ = h.Write(part)
	}

	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
