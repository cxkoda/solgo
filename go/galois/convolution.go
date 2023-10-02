package galois

import (
	"fmt"
	"io"
	"math/big"
	"math/bits"
)

// Convolve returns the convolution a∗b in log-linear time using the convolution
// theorem: both a and b are transformed to their frequency domain as A and B
// respectively, and the inverse-FFT of the dot product A•B is returned.
//
// The current FFT implementation only supports 2^k length slices. The
// primitiveRoot must therefore be an nth root of unity in the Field where n is
// the smallest power of 2 >= len(a)+len(b)-1; see f.ConvolutionRoot().This
// remains more efficient than a quadratic-time algorithm even with maximal
// padding.
func (f *Field) Convolve(a, b []*big.Int, primitiveRoot *big.Int) ([]*big.Int, error) {
	n := convolutionSize(a, b)
	aFreq, err := f.FFT(zeroPad(a, int(n)), primitiveRoot)
	if err != nil {
		return nil, fmt.Errorf(`%T.FFT("a" values): %v`, f, err)
	}
	bFreq, err := f.FFT(zeroPad(b, int(n)), primitiveRoot)
	if err != nil {
		return nil, fmt.Errorf(`%T.FFT("b" values): %v`, f, err)
	}

	product := aFreq
	for i, a := range product {
		a.Mul(a, bFreq[i])
	}
	result, err := f.FFTInverse(product, primitiveRoot)
	if err != nil {
		return nil, fmt.Errorf("%T.FFTInverse(…): %v", f, err)
	}
	return result[:len(a)+len(b)-1], nil
}

// ConvolutionRoot returns a primitive root of unity suitable for convolving a
// and b with f.Convolve. The Reader is propagated to f.RootOfUnity().
func (f *Field) ConvolutionRoot(r io.Reader, a, b []*big.Int) (*big.Int, error) {
	return f.RootOfUnity(r, convolutionSize(a, b), true)
}

// convolutionSize returns the smallest power of 2 >= len(a)+len(b)-1. This is
// required for the recursive splitting performed by the current FFT
// implementation.
func convolutionSize(a, b []*big.Int) uint64 {
	n := uint64(len(a) + len(b) - 1)
	if (n & (n - 1)) == 0 {
		return n
	}
	// next power of 2
	return 1 << (64 - bits.LeadingZeros64(n))
}

func zeroPad(x []*big.Int, size int) []*big.Int {
	pad := make([]*big.Int, size-len(x))
	for i := range pad {
		pad[i] = big.NewInt(0)
	}
	return append(x, pad...)
}
