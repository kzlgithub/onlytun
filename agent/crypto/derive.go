package crypto

import (
	"encoding/binary"

	"golang.org/x/crypto/blake2b"
)

const (
	labelEnc    = "OnlyTun-enc-v1"
	labelAuth   = "OnlyTun-auth-v1"
	labelConst  = "OnlyTun-const-v1"
	labelRot    = "OnlyTun-rot-v1"
	labelSBox   = "OnlyTun-sbox-v1"
	labelBitRot = "OnlyTun-bitrot-v1"
)

// DeriveSessionKeys 从 PSK 和握手随机数派生会话密钥。
func DeriveSessionKeys(psk, clientRandom, serverRandom []byte) (encKey, authKey []byte) {
	enc := deriveHash(labelEnc, psk, clientRandom, serverRandom)
	auth := deriveHash(labelAuth, psk, clientRandom, serverRandom)
	return enc[:], auth[:]
}

// DeriveCipherConstants 从 PSK 派生流密码初始常量。
func DeriveCipherConstants(psk []byte) [4]uint32 {
	sum := deriveHash(labelConst, psk)
	var out [4]uint32
	for i := 0; i < len(out); i++ {
		out[i] = binary.LittleEndian.Uint32(sum[i*4 : (i+1)*4])
	}
	return out
}

// DeriveRotationTable 从 PSK 派生轮函数旋转量。
func DeriveRotationTable(psk []byte) [4]int {
	sum := deriveHash(labelRot, psk)
	var out [4]int
	for i := 0; i < len(out); i++ {
		out[i] = int(sum[i]%23) + 5
	}
	return out
}

// DeriveSBoxSeed 从 PSK 派生 S-Box 生成种子。
func DeriveSBoxSeed(psk []byte) uint64 {
	sum := deriveHash(labelSBox, psk)
	return binary.LittleEndian.Uint64(sum[:8])
}

// DeriveBitRotBase 从 PSK 派生位旋转基础值。
func DeriveBitRotBase(psk []byte) byte {
	sum := deriveHash(labelBitRot, psk)
	return sum[0] & 0x07
}

func deriveHash(label string, parts ...[]byte) [32]byte {
	h, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}

	writeLengthPrefixed(h, []byte(label))
	for _, part := range parts {
		writeLengthPrefixed(h, part)
	}

	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func writeLengthPrefixed(h interface{ Write([]byte) (int, error) }, data []byte) {
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(len(data)))
	_, _ = h.Write(size[:])
	_, _ = h.Write(data)
}
