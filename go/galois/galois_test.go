package galois

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
)

func bigEqual(a, b *big.Int) bool {
	return a.Cmp(b) == 0
}

func TestRootOfUnity(t *testing.T) {
	// bn256.Order - 1 is a multiple of 2^28, so we can find nth roots of unity
	// for a range of 2^n.
	f := NewField(bn256.Order)

	for log2n := uint64(1); log2n < 14; log2n++ {
		t.Run(fmt.Sprintf("n = 2^%d", log2n), func(t *testing.T) {
			n := uint64(1) << log2n

			got, err := f.RootOfUnity(crypto.NewKeccakState(), n, true)
			if err != nil || got.Cmp(bigOne) == 0 {
				t.Fatalf("%T.RootOfUnity(…, 2^%d, true) got %d, err = %v; want root!=1, nil err", f, log2n, got, err)
			}

			bigN := new(big.Int).Lsh(bigOne, uint(log2n))

			for i := uint64(0); i < n; i++ {
				root := f.Exp(got, new(big.Int).SetUint64(i+1))
				if !bigEqual(f.Exp(root, bigN), big.NewInt(1)) {
					t.Fatalf("%T.RootOfUnity(…, primitive=true)^%d^n != 1 mod q", f, i+1)
				}
			}
		})
	}
}

func TestMultInverse(t *testing.T) {
	f := NewField(bn256.Order)
	rng := crypto.NewKeccakState()

	for i := 0; i < 20; i++ {
		x, err := f.Random(rng)
		if err != nil {
			t.Fatalf("%T.Random(%T) error %v", f, rng, err)
		}

		inv := f.MultInverse(x)
		if !bigEqual(f.Mul(x, inv), bigOne) {
			t.Errorf("%T.MultInverse(%x) got %x; want multiplicative inverse", f, x, inv)
		}
	}
}
