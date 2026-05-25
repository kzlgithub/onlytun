package obfs

import otcrypto "github.com/onlytun/agent/crypto"

// SBox 私有字节置换表。
type SBox struct {
	table    [256]byte
	invTable [256]byte
}

// NewSBox 从 PSK 生成私有 S-Box。
func NewSBox(psk []byte) *SBox {
	s := &SBox{}
	for i := 0; i < len(s.table); i++ {
		s.table[i] = byte(i)
	}

	rng := newSplitMix64(otcrypto.DeriveSBoxSeed(psk))
	for i := len(s.table) - 1; i > 0; i-- {
		j := int(rng.next() % uint64(i+1))
		s.table[i], s.table[j] = s.table[j], s.table[i]
	}

	for i, value := range s.table {
		s.invTable[value] = byte(i)
	}

	return s
}

// Obfuscate 对数据应用 S-Box 正向置换。
func (s *SBox) Obfuscate(data []byte) {
	for i, value := range data {
		data[i] = s.table[value]
	}
}

// Deobfuscate 对数据应用 S-Box 逆向置换。
func (s *SBox) Deobfuscate(data []byte) {
	for i, value := range data {
		data[i] = s.invTable[value]
	}
}

type splitMix64 struct {
	state uint64
}

func newSplitMix64(seed uint64) *splitMix64 {
	return &splitMix64{state: seed}
}

func (s *splitMix64) next() uint64 {
	s.state += 0x9e3779b97f4a7c15
	z := s.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
