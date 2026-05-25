package obfs

import (
	"math/bits"

	otcrypto "github.com/onlytun/agent/crypto"
)

// BitRotObfs 位旋转混淆器。
type BitRotObfs struct {
	baseRot byte
}

// NewBitRotObfs 从 PSK 创建位旋转混淆器。
func NewBitRotObfs(psk []byte) *BitRotObfs {
	return &BitRotObfs{
		baseRot: otcrypto.DeriveBitRotBase(psk),
	}
}

// Obfuscate 对数据应用位旋转混淆。
func (b *BitRotObfs) Obfuscate(data []byte, counter uint64) {
	rot := b.startRot(counter)
	for i, value := range data {
		data[i] = bits.RotateLeft8(value, int(rot))
		rot = (rot + 3) & 0x07
	}
}

// Deobfuscate 对数据应用位旋转解混淆。
func (b *BitRotObfs) Deobfuscate(data []byte, counter uint64) {
	rot := b.startRot(counter)
	for i, value := range data {
		data[i] = bits.RotateLeft8(value, -int(rot))
		rot = (rot + 3) & 0x07
	}
}

func (b *BitRotObfs) startRot(counter uint64) byte {
	mix := byte(counter) ^ byte(counter>>8) ^ byte(counter>>16) ^ byte(counter>>24) ^
		byte(counter>>32) ^ byte(counter>>40) ^ byte(counter>>48) ^ byte(counter>>56)
	return (b.baseRot + mix) & 0x07
}
