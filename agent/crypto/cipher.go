package crypto

import (
	"encoding/binary"
	"errors"
	"math"
	"math/bits"
)

const (
	pskSize   = 32
	keySize   = 32
	nonceSize = 12
	blockSize = 64
)

// PrivateCipher 私有流密码，参数从 PSK 派生。
type PrivateCipher struct {
	rotations    [4]int
	initialState [16]uint32
	state        [16]uint32
	block        [blockSize]byte
	offset       int
	exhausted    bool
}

// NewPrivateCipher 创建私有流密码实例。
func NewPrivateCipher(psk, key, nonce []byte) (*PrivateCipher, error) {
	switch {
	case len(psk) != pskSize:
		return nil, errors.New("crypto: invalid PSK length")
	case len(key) != keySize:
		return nil, errors.New("crypto: invalid key length")
	case len(nonce) != nonceSize:
		return nil, errors.New("crypto: invalid nonce length")
	}

	c := &PrivateCipher{
		rotations: DeriveRotationTable(psk),
		offset:    blockSize,
	}

	constants := DeriveCipherConstants(psk)
	copy(c.initialState[0:4], constants[:])

	for i := 0; i < 8; i++ {
		c.initialState[4+i] = binary.LittleEndian.Uint32(key[i*4 : (i+1)*4])
	}

	c.initialState[12] = 0
	c.initialState[13] = binary.LittleEndian.Uint32(nonce[0:4])
	c.initialState[14] = binary.LittleEndian.Uint32(nonce[4:8])
	c.initialState[15] = binary.LittleEndian.Uint32(nonce[8:12])

	c.Reset()
	return c, nil
}

// XORKeyStream 对数据进行加密或解密。
// It panics if dst and src lengths differ or the keystream is exhausted.
func (c *PrivateCipher) XORKeyStream(dst, src []byte) {
	if len(dst) != len(src) {
		panic("crypto: dst and src length mismatch")
	}

	for len(src) > 0 {
		if c.offset == blockSize {
			c.generateBlock()
		}

		n := blockSize - c.offset
		if n > len(src) {
			n = len(src)
		}

		for i := 0; i < n; i++ {
			dst[i] = src[i] ^ c.block[c.offset+i]
		}

		c.offset += n
		dst = dst[n:]
		src = src[n:]
	}
}

// Reset restores the cipher to its original key/nonce state.
// It must not be used to encrypt different messages with the same key+nonce.
func (c *PrivateCipher) Reset() {
	c.state = c.initialState
	for i := range c.block {
		c.block[i] = 0
	}
	c.offset = blockSize
	c.exhausted = false
}

func (c *PrivateCipher) generateBlock() {
	if c.exhausted {
		panic("crypto: keystream exhausted")
	}

	working := c.state

	for i := 0; i < 10; i++ {
		c.quarterRound(&working[0], &working[4], &working[8], &working[12])
		c.quarterRound(&working[1], &working[5], &working[9], &working[13])
		c.quarterRound(&working[2], &working[6], &working[10], &working[14])
		c.quarterRound(&working[3], &working[7], &working[11], &working[15])

		c.quarterRound(&working[0], &working[5], &working[10], &working[15])
		c.quarterRound(&working[1], &working[6], &working[11], &working[12])
		c.quarterRound(&working[2], &working[7], &working[8], &working[13])
		c.quarterRound(&working[3], &working[4], &working[9], &working[14])
	}

	for i := range working {
		working[i] += c.state[i]
		binary.LittleEndian.PutUint32(c.block[i*4:(i+1)*4], working[i])
	}

	if c.state[12] == math.MaxUint32 {
		c.exhausted = true
	} else {
		c.state[12]++
	}
	c.offset = 0
}

func (c *PrivateCipher) quarterRound(a, b, cWord, d *uint32) {
	*a += *b
	*d ^= *a
	*d = bits.RotateLeft32(*d, c.rotations[0])

	*cWord += *d
	*b ^= *cWord
	*b = bits.RotateLeft32(*b, c.rotations[1])

	*a += *b
	*d ^= *a
	*d = bits.RotateLeft32(*d, c.rotations[2])

	*cWord += *d
	*b ^= *cWord
	*b = bits.RotateLeft32(*b, c.rotations[3])
}
