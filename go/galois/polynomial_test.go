package galois

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
	"github.com/google/go-cmp/cmp"
)

func TestPolynomialFromRoots(t *testing.T) {
	f := NewField(bn256.Order)

	tests := []struct {
		roots, want []int64
	}{
		{
			roots: []int64{99},
			want:  []int64{-99, 1},
		},
		{
			roots: []int64{42},
			want:  []int64{-42, 1},
		},
		{
			roots: []int64{1, 2, 3, 4, 5, 6},
			want:  []int64{720, -1764, 1624, -735, 175, -21, 1},
		},
		{
			roots: []int64{72, -176, 162, -73, 17, -21, 1},
			want:  []int64{53499688704, -54135573264, 474383694, 161743333, -208416, -34070, 18, 1},
		},
		{
			roots: []int64{3, 7, 13, 19, 23},
			want:  []int64{-119301, 77453, -16666, 1554, -65, 1},
		},
	}

	for _, tt := range tests {
		roots := asBig(tt.roots)
		want := asBig(tt.want)
		for _, w := range want {
			w.Mod(w, f.Order())
		}

		got, err := f.PolynomialFromRoots(roots)
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range got {
			g.Mod(g, f.Order())
		}

		if diff := cmp.Diff(want, got, cmp.Comparer(bigEqual)); diff != "" {
			t.Error(diff)
		}
	}
}

func BenchmarkPolynomailFromRoots(b *testing.B) {
	f := NewField(bn256.Order)

	rng := crypto.NewKeccakState()
	buf := make([]byte, 20)
	roots := make([]*big.Int, 100)
	for i := range roots {
		if _, err := rng.Read(buf); err != nil {
			b.Fatal(err)
		}
		roots[i] = new(big.Int).SetBytes(buf)
	}

	for i := 0; i < b.N; i++ {
		if _, err := f.PolynomialFromRoots(roots); err != nil {
			b.Fatal(err)
		}
	}
}
