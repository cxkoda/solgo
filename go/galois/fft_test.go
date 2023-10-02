package galois

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/go-cmp/cmp"
)

func TestFFTvsNaiveDFT(t *testing.T) {
	const n = 16

	q := int64(n)*15 + 1
	f := NewField(big.NewInt(q))
	rng := crypto.NewKeccakState()

	root, err := f.RootOfUnity(rng, n, true)
	if err != nil {
		t.Fatalf("%T.RootOfUnity(%T, %d, true) error %v", f, rng, n, err)
	}

	tests := make([][n]*big.Int, 50)

	for i := 0; i < n; i++ {
		tests[0][i] = big.NewInt(int64(i))
		tests[1][i] = big.NewInt(int64(n - i))
		for j := 2; j < len(tests); j++ {
			x, err := f.Random(rng)
			if err != nil {
				t.Fatalf("%T.Random(%T) error %v", f, rng, err)
			}
			tests[j][i] = x
		}
	}

	for _, tt := range tests {
		time := tt[:]

		freq := f.dft(t, time, root)
		got := f.iDFT(t, freq, root)
		if !cmp.Equal(time, got, cmp.Comparer(bigEqual)) {
			t.Fatalf("iDFT(dft(%d)) got %d; want original", tt, got)
		}

		t.Run("FFT", func(t *testing.T) {
			want := freq
			got, err := f.FFT(time, root)
			if err != nil {
				t.Fatalf("%T.FFT(%d, %d) error %v", f, time, root, err)
			}
			if !cmp.Equal(got, want, cmp.Comparer(bigEqual)) {
				t.Errorf("%T.FFT(%d, %d) got %d; want %d (computed by naive DFT)", f, time, root, got, want)
			}

			t.Run("Inverse", func(t *testing.T) {
				got, err := f.FFTInverse(freq, root)
				if err != nil {
					t.Fatalf("%T.FFTInverse(%d, %d) error %v", f, freq, root)
				}
				if want := time; !cmp.Equal(got, want, cmp.Comparer(bigEqual)) {
					t.Errorf("%T.FFTInverse(%d, %d) got %d; want %d", f, freq, root, got, want)
				}
			})
		})
	}
}

// dft implements a naive O(n^2) DFT to test FFT implementations.
func (f *Field) dft(t *testing.T, xs []*big.Int, root *big.Int) []*big.Int {
	t.Helper()
	roots, err := f.twiddleFactors(xs, root)
	if err != nil {
		t.Fatalf("dft(): %T.twiddleFactors(%d, %d) error %v", f, xs, root, err)
	}

	X := make([]*big.Int, len(xs))
	for k := range xs {
		X[k] = big.NewInt(0)
		for n, x := range xs {
			r := (-k * n) % len(xs)
			if r < 0 {
				r += len(xs)
			}
			X[k].Add(X[k], new(big.Int).Mul(x, roots[r]))
		}
		X[k].Mod(X[k], f.Order())
	}
	return X
}

func (f *Field) iDFT(t *testing.T, Xs []*big.Int, root *big.Int) []*big.Int {
	t.Helper()
	roots, err := f.twiddleFactors(Xs, root)
	if err != nil {
		t.Fatalf("dft(): %T.twiddleFactors(%d, %d) error %v", f, Xs, root, err)
	}

	inv := f.MultInverse(big.NewInt(int64(len(Xs))))

	x := make([]*big.Int, len(Xs))
	for n := range Xs {
		x[n] = big.NewInt(0)
		for k, X := range Xs {
			x[n].Add(x[n], new(big.Int).Mul(X, roots[(k*n)%len(Xs)]))
		}
		x[n].Mul(x[n], inv)
		x[n].Mod(x[n], f.Order())
	}
	return x
}
