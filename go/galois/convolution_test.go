package galois

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
	"github.com/google/go-cmp/cmp"
)

func TestConvolve(t *testing.T) {
	f := NewField(bn256.Order)

	type test struct {
		name       string
		a, b, want []int64
	}

	tests := []test{
		{
			name: "unity",
			a:    []int64{1},
			b:    []int64{5, 9, 13, 14, 2, -8},
			want: []int64{5, 9, 13, 14, 2, -8},
		},
		{
			name: "constant",
			a:    []int64{3},
			b:    []int64{5, 9, 13, 14, 2, -8},
			want: []int64{5 * 3, 9 * 3, 13 * 3, 14 * 3, 2 * 3, -8 * 3},
		},
		{
			// https://www.symbolab.com/solver/polynomial-multiplication-calculator/%28x%5E2%2B2x-1%29%5Ccdot%282x%5E2-3x%2B6%29?or=ex
			name: "arbitrary",
			a:    []int64{1, 2, -1},
			b:    []int64{2, -3, 6},
			want: []int64{2, 1, -2, 15, -6},
		},
		{
			name: "arbitrary reversed",
			a:    []int64{-1, 2, 1},
			b:    []int64{6, -3, 2},
			want: []int64{-6, 15, -2, 1, 2},
		},
		{
			// https://www.symbolab.com/solver/polynomial-multiplication-calculator/%5Cleft(-4x%5E%7B3%7D%2B2x%5E%7B2%7D-7x-7%5Cright)%5Ccdot%5Cleft(8x%5E%7B6%7D%2B3x%5E%7B5%7D-12x%5E%7B4%7D%2Bx%5E%7B3%7D%2Bx%5E%7B2%7D-13x%2B9%5Cright)?or=input
			name: "longer arbitrary",
			a:    []int64{-4, 2, -7, -7},
			b:    []int64{8, 3, -12, 1, 1, -13, 9},
			want: []int64{-32, 4, -2, -105, 61, 131, -76, 102, 28, -63},
		},
	}

	for i, n := 0, len(tests); i < n; i++ {
		tt := tests[i]
		tests = append(tests, test{
			name: fmt.Sprintf("%s (commutative)", tt.name),
			a:    tt.b,
			b:    tt.a,
			want: tt.want,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := asBig(tt.a)
			b := asBig(tt.b)
			want := asBig(tt.want)
			for _, w := range want {
				w.Mod(w, f.Order())
			}

			rng := crypto.NewKeccakState()
			root, err := f.ConvolutionRoot(rng, a, b)
			if err != nil {
				t.Fatalf("%T.ConvolutionRoot(%T, [len %d], [len %d]) error %v", f, rng, len(a), len(b), err)
			}
			got, err := f.Convolve(a, b, root)
			if err != nil {
				t.Errorf("%T.Convolve(%T, %d, %d) error %v", f, rng, a, b, err)
			}
			if diff := cmp.Diff(got, want, cmp.Comparer(bigEqual)); diff != "" {
				t.Errorf("%T.Convolve(%T, %d, %d) diff (-want +got):\n%s", f, rng, a, b, diff)
			}
		})
	}
}

func asBig(xs []int64) []*big.Int {
	b := make([]*big.Int, len(xs))
	for i, x := range xs {
		b[i] = big.NewInt(x)
	}
	return b
}
