package galois

import (
	"errors"
	"fmt"
	"math/big"
)

// FFT performs a fast Fourier transform of xs. The nth primitiveRoot (of unity)
// can be obtained with f.RootOfUnity(…, len(xs), true).
func (f *Field) FFT(x []*big.Int, primitiveRoot *big.Int) ([]*big.Int, error) {
	return f.fft(x, primitiveRoot, false)
}

// FFTInverse inverts f.FFT().
func (f *Field) FFTInverse(X []*big.Int, primitiveRoot *big.Int) ([]*big.Int, error) {
	time, err := f.fft(X, primitiveRoot, true)
	if err != nil {
		return nil, err
	}

	inv := f.MultInverse(big.NewInt(int64(len(X))))
	for i, t := range time {
		// TODO(aschlosberg): methods on Field that allow setting of a big.Int
		// instead of always creating a new one. Something like:
		// f.Set(t).Mul(t,inv).
		time[i] = f.Mul(t, inv)
	}
	return time, nil
}

// fft implements FFT and its inverse.
func (f *Field) fft(xs []*big.Int, primitiveRoot *big.Int, inverse bool) ([]*big.Int, error) {
	roots, err := f.twiddleFactors(xs, primitiveRoot)
	if err != nil {
		return nil, err
	}

	freq := f.cooleyTukey(xs, roots, 1, inverse)
	for _, fr := range freq {
		fr.Mod(fr, f.Order())
		fr.Abs(fr)
	}
	return freq, nil
}

// cooleyTukey implements the Cooley–Tukey FFT algorithm
// https://en.wikipedia.org/wiki/Cooley%E2%80%93Tukey_FFT_algorithm.
func (f *Field) cooleyTukey(xs, roots []*big.Int, stride int, inverse bool) []*big.Int {
	if len(xs) == 1 {
		return []*big.Int{new(big.Int).Set(xs[0])}
	}

	even := make([]*big.Int, len(xs)/2)
	odd := make([]*big.Int, len(even))
	for i := range even {
		even[i] = xs[i*2]
		odd[i] = xs[i*2+1]
	}
	freqEven := f.cooleyTukey(even, roots, stride*2, inverse)
	freqOdd := f.cooleyTukey(odd, roots, stride*2, inverse)

	halfLen := len(xs) / 2

	freq := make([]*big.Int, len(xs))
	var root int
	for k, e := range freqEven {
		if inverse {
			root = (k * stride) % len(roots)
		} else {
			root = (-k * stride) % len(roots)
			if root < 0 {
				root += len(roots)
			}
		}

		o := freqOdd[k]
		o.Mul(o, roots[root])

		freq[k] = new(big.Int).Add(e, o)
		freq[k+halfLen] = new(big.Int).Sub(e, o)
	}
	return freq
}

func (f *Field) twiddleFactors(xs []*big.Int, root *big.Int) ([]*big.Int, error) {
	n := uint64(len(xs))
	if n&(n-1) != 0 {
		return nil, errors.New("FFT on non-power-of-2 values unimplemented")
	}
	if root.Cmp(bigOne) == 0 {
		return nil, errors.New("FFT root of unity == 1")
	}
	if f.Exp(root, new(big.Int).SetUint64(n)).Cmp(bigOne) != 0 {
		return nil, fmt.Errorf("primitive root 0x%x is not a %d(th) root of unity", root, n)
	}

	roots := make([]*big.Int, len(xs))
	for i := range xs {
		roots[i] = f.Exp(root, big.NewInt(int64(i)))
	}

	return roots, nil
}
