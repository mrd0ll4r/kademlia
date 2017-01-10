package kademlia

import (
	"math/big"
	"testing"
)

var xorTestCases = []struct {
	a        [20]byte
	b        [20]byte
	expected *big.Int
}{
	{
		[20]byte{},
		[20]byte{},
		big.NewInt(0),
	},
	{
		[20]byte{},
		[20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		big.NewInt(1),
	},
	{
		[20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		[20]byte{},
		big.NewInt(1),
	},
	{
		[20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		[20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		big.NewInt(0),
	},
	{
		[20]byte{},
		[20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0},
		big.NewInt(1 << 8),
	},
	{
		[20]byte{},
		[20]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		big.NewInt(0).Lsh(big.NewInt(1), 152),
	},
	{
		[20]byte{},
		[20]byte{128, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		big.NewInt(0).Lsh(big.NewInt(1), 159),
	},
	{
		[20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		[20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0},
		big.NewInt(1<<8 + 1),
	},
}

func TestXorDist(t *testing.T) {
	for _, c := range xorTestCases {
		res := XorDist(c.a, c.b)
		if res.Cmp(c.expected) != 0 {
			t.Errorf("XORDist(%x, %x): expected %s, got %s", c.a, c.b, c.expected, res)
		}
	}
}
