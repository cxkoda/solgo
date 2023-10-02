// Package shuffle provides mechanisms for permuting data.
package shuffle

import (
	"fmt"
)

// FisherYates implements a Fisher–Yates shuffle with an extendable pool size.
type FisherYates struct {
	perm     []int
	shuffled int
}

// NewFisherYates returns a new FisherYates with a specified pool size.
func NewFisherYates(n uint32) *FisherYates {
	perm := make([]int, int(n))
	for i := range perm {
		perm[i] = i
	}
	return &FisherYates{perm: perm}
}

// Permutation returns the already-permuted values.
func (f *FisherYates) Permutation() []int {
	return append([]int{}, f.perm[:f.shuffled]...)
}

// A Rand can return a random integer from a range. It is typically provided by
// a math/rand.Rand pointer, but is defined as an interface to allow for
// alternatives such as piping crypto/rand into a custom type.
type Rand interface {
	// Intn returns an integer in [0,n).
	Intn(n int) int
}

// PermuteUpTo is equivalent to Permute(min(n, f.Remaining())).
func (f *FisherYates) PermuteUpTo(n uint32, rng Rand) []int {
	if r := f.Remaining(); n > r {
		n = r
	}

	x, err := f.Permute(n, rng)
	if err != nil {
		panic(fmt.Errorf("BUG: %v", err))
	}
	return x
}

// permuteTooMany is an error returned by Permute(n) if n is greater than the
// number of unshuffled values in the pool.
type permuteTooMany struct {
	n, remain uint32
}

func (p *permuteTooMany) Error() string {
	return fmt.Sprintf("%T.Permute(%d) with only %d unshuffled", &FisherYates{}, p.n, p.remain)
}

// Permute chooses up to n values from the pool, limited by the number of
// already-permuted values.
func (f *FisherYates) Permute(n uint32, rng Rand) ([]int, error) {
	if r := f.Remaining(); n > r {
		return nil, &permuteTooMany{
			n:      n,
			remain: r,
		}
	}
	max := f.shuffled + int(n)

	for i := f.shuffled; i < max; i++ {
		f.swap(i, rng.Intn(len(f.perm)-i)+i)
	}
	chosen := f.perm[f.shuffled:max]
	f.shuffled = max

	return chosen, nil
}

func (f *FisherYates) swap(a, b int) {
	f.perm[a], f.perm[b] = f.perm[b], f.perm[a]
}

// Grow increases the size of the pool of values from which Permute can choose.
// It doesn't change already-permuted values in any way, and simply appends the
// next n indices to those available to future calls to Permute.
func (f *FisherYates) Grow(delta uint32) {
	size := len(f.perm)
	f.perm = append(f.perm, make([]int, int(delta))...)
	for i := size; i < len(f.perm); i++ {
		f.perm[i] = i
	}
}

// GrowTo is equivalent to f.Grow(max(0, f.Size()-n)); i.e. it grows the size of
// the pool to n, unless it is already greater than or equal to this size.
func (f *FisherYates) GrowTo(n uint32) {
	sz := uint32(len(f.perm))
	if n < sz {
		return
	}
	f.Grow(n - sz)
}

// Size returns the size of the pool from which Permute() chooses the next
// indices. It is the sum of the values passed to NewFisherYates() and all calls
// to Grow().
func (f *FisherYates) Size() int {
	return len(f.perm)
}

// Remaining returns the number of unshuffled values in the pool; i.e. the
// difference between Size() and the sum of all n passed to Permute().
func (f *FisherYates) Remaining() uint32 {
	return uint32(len(f.perm) - f.shuffled)
}
