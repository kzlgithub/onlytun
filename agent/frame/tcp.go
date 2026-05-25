package frame

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	otcrypto "github.com/onlytun/agent/crypto"
	"github.com/onlytun/agent/obfs"
)

const (
	maxPrefixLen = 31
	macSize      = 16
	readChunk    = 1024
)

var (
	errInvalidNonceSize = errors.New("frame: invalid nonce size")
	errPayloadTooLarge  = errors.New("frame: payload too large")
	errAuthFailed       = errors.New("frame: authentication failed")
	errInvalidPrefixLen = errors.New("frame: invalid prefix length")
)

// TCPFramer encodes and decodes OnlyTun TCP stream frames.
type TCPFramer struct {
	writeEncKey  []byte
	writeAuthKey []byte
	readEncKey   []byte
	readAuthKey  []byte
	baseRot      byte
	writeCtr     uint64
	readCtr      uint64

	writeCipher *otcrypto.PrivateCipher
	readCipher  *otcrypto.PrivateCipher
	sbox        *obfs.SBox
	bitrot      *obfs.BitRotObfs

	rawReadBuf []byte

	frameObfs  []byte
	frameCrypt []byte
	framePlain []byte
	needBody   int
}

// NewTCPFramer creates a TCP framer with isolated write/read cipher state.
func NewTCPFramer(psk, writeEncKey, writeAuthKey, readEncKey, readAuthKey, nonce []byte) (*TCPFramer, error) {
	if len(nonce) != 12 {
		return nil, errInvalidNonceSize
	}

	writeCipher, err := otcrypto.NewPrivateCipher(psk, writeEncKey, nonce)
	if err != nil {
		return nil, fmt.Errorf("frame: create write cipher: %w", err)
	}
	readCipher, err := otcrypto.NewPrivateCipher(psk, readEncKey, nonce)
	if err != nil {
		return nil, fmt.Errorf("frame: create read cipher: %w", err)
	}

	return &TCPFramer{
		writeEncKey:  append([]byte(nil), writeEncKey...),
		writeAuthKey: append([]byte(nil), writeAuthKey...),
		readEncKey:   append([]byte(nil), readEncKey...),
		readAuthKey:  append([]byte(nil), readAuthKey...),
		baseRot:      otcrypto.DeriveBitRotBase(psk),
		writeCipher:  writeCipher,
		readCipher:   readCipher,
		sbox:         obfs.NewSBox(psk),
		bitrot:       obfs.NewBitRotObfs(psk),
		needBody:     -1,
	}, nil
}

// WriteFrame wraps a payload into a protected TCP frame and writes it fully.
func (f *TCPFramer) WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) > 0xffff {
		return errPayloadTooLarge
	}

	counter := f.writeCtr
	prefixLen, prefix, err := randomPrefix()
	if err != nil {
		return fmt.Errorf("frame: random prefix: %w", err)
	}

	body := make([]byte, 1+prefixLen+2+len(payload))
	body[0] = byte(prefixLen)
	copy(body[1:], prefix)

	offset := 1 + prefixLen
	encodedLen := uint16(len(payload)) ^ lengthMask(f.writeEncKey, counter)
	binary.LittleEndian.PutUint16(body[offset:offset+2], encodedLen)
	copy(body[offset+2:], payload)

	ciphertext := make([]byte, len(body))
	f.writeCipher.XORKeyStream(ciphertext, body)

	mac, err := otcrypto.ComputeMAC(f.writeAuthKey, counter, ciphertext)
	if err != nil {
		return fmt.Errorf("frame: compute mac: %w", err)
	}

	obfsBody := append([]byte(nil), ciphertext...)
	f.sbox.Obfuscate(obfsBody)
	f.bitrot.Obfuscate(obfsBody, counter)

	obfsMAC := append([]byte(nil), mac[:]...)
	f.sbox.Obfuscate(obfsMAC)
	f.bitrot.Obfuscate(obfsMAC, counter)

	packet := make([]byte, len(obfsBody)+len(obfsMAC))
	copy(packet, obfsBody)
	copy(packet[len(obfsBody):], obfsMAC)

	if err := writeFull(w, packet); err != nil {
		return err
	}

	f.writeCtr++
	return nil
}

func writeFull(w io.Writer, buf []byte) error {
	for len(buf) > 0 {
		n, err := w.Write(buf)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		buf = buf[n:]
	}
	return nil
}

// ReadFrame reads and verifies a single TCP frame from a stream.
func (f *TCPFramer) ReadFrame(r io.Reader) ([]byte, error) {
	for {
		if f.needBody >= 0 && len(f.frameObfs) >= f.needBody+macSize {
			payload, err := f.finalizeFrame()
			if err != nil {
				return nil, err
			}
			return payload, nil
		}

		b, err := f.readRawByte(r)
		if err != nil {
			return nil, err
		}

		f.frameObfs = append(f.frameObfs, b)
		if f.needBody >= 0 && len(f.frameObfs) > f.needBody {
			continue
		}

		cipherByte := f.deobfuscateBodyByte(b, len(f.frameCrypt))
		f.frameCrypt = append(f.frameCrypt, cipherByte)

		var plainByte [1]byte
		f.readCipher.XORKeyStream(plainByte[:], []byte{cipherByte})
		f.framePlain = append(f.framePlain, plainByte[0])

		if f.needBody < 0 {
			if err := f.tryResolveBodyLength(); err != nil {
				return nil, err
			}
		}
	}
}

func (f *TCPFramer) tryResolveBodyLength() error {
	if len(f.framePlain) < 1 {
		return nil
	}

	prefixLen := int(f.framePlain[0])
	if prefixLen > maxPrefixLen {
		return errInvalidPrefixLen
	}

	lengthOffset := 1 + prefixLen
	if len(f.framePlain) < lengthOffset+2 {
		return nil
	}

	encodedLen := binary.LittleEndian.Uint16(f.framePlain[lengthOffset : lengthOffset+2])
	payloadLen := encodedLen ^ lengthMask(f.readEncKey, f.readCtr)
	f.needBody = 1 + prefixLen + 2 + int(payloadLen)
	return nil
}

func (f *TCPFramer) finalizeFrame() ([]byte, error) {
	bodyLen := f.needBody
	ciphertext := append([]byte(nil), f.frameCrypt[:bodyLen]...)

	var mac [macSize]byte
	copy(mac[:], f.frameObfs[bodyLen:bodyLen+macSize])
	f.bitrot.Deobfuscate(mac[:], f.readCtr)
	f.sbox.Deobfuscate(mac[:])

	if !otcrypto.VerifyMAC(f.readAuthKey, f.readCtr, ciphertext, mac) {
		return nil, errAuthFailed
	}

	prefixLen := int(f.framePlain[0])
	lengthOffset := 1 + prefixLen
	payloadLen := binary.LittleEndian.Uint16(f.framePlain[lengthOffset:lengthOffset+2]) ^ lengthMask(f.readEncKey, f.readCtr)
	payloadStart := lengthOffset + 2
	payload := append([]byte(nil), f.framePlain[payloadStart:payloadStart+int(payloadLen)]...)

	f.readCtr++
	f.frameObfs = f.frameObfs[:0]
	f.frameCrypt = f.frameCrypt[:0]
	f.framePlain = f.framePlain[:0]
	f.needBody = -1

	return payload, nil
}

func lengthMask(encKey []byte, counter uint64) uint16 {
	offset := int(counter % 30)
	return binary.LittleEndian.Uint16(encKey[offset : offset+2])
}

func (f *TCPFramer) readRawByte(r io.Reader) (byte, error) {
	if len(f.rawReadBuf) == 0 {
		buf := make([]byte, readChunk)
		n, err := r.Read(buf)
		if n > 0 {
			f.rawReadBuf = append(f.rawReadBuf, buf[:n]...)
		}
		if err != nil {
			if err == io.EOF && len(f.rawReadBuf) > 0 {
				// Consume buffered bytes before returning EOF.
			} else {
				return 0, err
			}
		}
		if len(f.rawReadBuf) == 0 {
			return 0, io.EOF
		}
	}

	b := f.rawReadBuf[0]
	f.rawReadBuf = f.rawReadBuf[1:]
	return b, nil
}

func (f *TCPFramer) deobfuscateBodyByte(obfsByte byte, pos int) byte {
	rot := (f.bitrotBase(f.readCtr) + byte((pos*3)&0x07)) & 0x07
	plainRot := rotateRight8(obfsByte, rot)
	return f.sboxInverse(plainRot)
}

func (f *TCPFramer) bitrotBase(counter uint64) byte {
	mix := byte(counter) ^ byte(counter>>8) ^ byte(counter>>16) ^ byte(counter>>24) ^
		byte(counter>>32) ^ byte(counter>>40) ^ byte(counter>>48) ^ byte(counter>>56)
	return (f.baseRot + mix) & 0x07
}

func (f *TCPFramer) sboxInverse(v byte) byte {
	var one [1]byte
	one[0] = v
	f.sbox.Deobfuscate(one[:])
	return one[0]
}

func rotateRight8(v, rot byte) byte {
	rot &= 0x07
	if rot == 0 {
		return v
	}
	return (v >> rot) | (v << (8 - rot))
}

func randomPrefix() (int, []byte, error) {
	var prefixLenByte [1]byte
	if _, err := rand.Read(prefixLenByte[:]); err != nil {
		return 0, nil, err
	}

	prefixLen := int(prefixLenByte[0] & maxPrefixLen)
	prefix := make([]byte, prefixLen)
	if _, err := rand.Read(prefix); err != nil {
		return 0, nil, err
	}

	return prefixLen, prefix, nil
}
