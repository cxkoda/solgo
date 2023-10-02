package galois

import (
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
)

func (f *Field) PolynomialFromRoots(roots []*big.Int) ([]*big.Int, error) {
	n := convolutionSize(roots, []*big.Int{nil, nil})

	prim, err := f.RootOfUnity(crypto.NewKeccakState(), n, true)
	if err != nil {
		return nil, err
	}
	unityRoots, err := f.twiddleFactors(n, prim)
	if err != nil {
		return nil, err
	}

	return f.polyFromRoots(roots, unityRoots, n, 1), nil
}

func (f *Field) polyFromRoots(polyRoots, unityRoots []*big.Int, n uint64, stride int) []*big.Int {
	switch len(polyRoots) {
	case 1:
		return []*big.Int{new(big.Int).Neg(polyRoots[0]), big.NewInt(1)}
	case 2:
		sum := new(big.Int).Add(polyRoots[0], polyRoots[1])
		return []*big.Int{
			new(big.Int).Mul(polyRoots[0], polyRoots[1]),
			sum.Neg(sum),
			big.NewInt(1),
		}
	}

	half := len(polyRoots) / 2
	left := f.polyFromRoots(polyRoots[:half], unityRoots, n, stride)
	right := f.polyFromRoots(polyRoots[half:], unityRoots, n, stride)

	return f.convolve(
		zeroPad(left, int(n)),
		zeroPad(right, int(n)),
		unityRoots,
		len(polyRoots)+1,
		stride,
	)
}
