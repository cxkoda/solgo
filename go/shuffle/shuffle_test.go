package shuffle

import (
	"errors"
	"math"
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func diffOpts() cmp.Options {
	return cmp.Options{
		cmp.AllowUnexported(FisherYates{}),
		cmpopts.EquateEmpty(),
	}
}

func (f *FisherYates) mustPermute(t *testing.T, n uint32, rng Rand) []int {
	t.Helper()
	chosen, err := f.Permute(n, rng)
	if err != nil {
		t.Fatalf("%T.Permute(%d, …), error %v", f, n, err)
	}
	return chosen
}

func TestFisherYatesGrow(t *testing.T) {
	type growth struct {
		n        uint32
		want     *FisherYates
		wantSize int
	}

	tests := []struct {
		start   uint32
		want    *FisherYates
		growths []growth
	}{
		{
			start: 0,
			want:  &FisherYates{},
			growths: []growth{
				{
					n:        0,
					want:     &FisherYates{},
					wantSize: 0,
				},
				{
					n: 3,
					want: &FisherYates{
						perm: []int{0, 1, 2},
					},
					wantSize: 3,
				},
				{
					n: 2,
					want: &FisherYates{
						perm: []int{0, 1, 2, 3, 4},
					},
					wantSize: 5,
				},
				{
					n: 0,
					want: &FisherYates{
						perm: []int{0, 1, 2, 3, 4},
					},
					wantSize: 5,
				},
			},
		},
		{
			start: 1,
			want: &FisherYates{
				perm: []int{0},
			},
			growths: []growth{
				{
					n: 2,
					want: &FisherYates{
						perm: []int{0, 1, 2},
					},
					wantSize: 3,
				},
			},
		},
		{
			start: 6,
			want: &FisherYates{
				perm: []int{0, 1, 2, 3, 4, 5},
			},
			growths: []growth{
				{
					n: 4,
					want: &FisherYates{
						perm: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
					},
					wantSize: 10,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := NewFisherYates(tt.start)
			if diff := cmp.Diff(tt.want, got, diffOpts()); diff != "" {
				t.Fatalf("NewFisherYates(%d) diff (-want +got):\n%s", tt.start, diff)
			}
			if got, want := got.Size(), int(tt.start); got != want {
				t.Errorf("NewFisherYates(%d).Size() got %d; want %d", tt.start, got, want)
			}

			var growths []uint32
			for _, g := range tt.growths {
				growths = append(growths, g.n)

				got.Grow(g.n)
				if diff := cmp.Diff(g.want, got, diffOpts()); diff != "" {
					t.Fatalf("NewFisherYates(%d) after calls to Grow() with values %d; diff (-want +got):\n%s", tt.start, growths, diff)
				}
				if got, want := got.Size(), g.wantSize; got != want {
					t.Fatalf("NewFisherYates(%d).Size() after calls to Grow() with values %d; got %d; want %d", tt.start, growths, got, want)
				}
			}
		})
	}
}

type constRand int

// Intn returns r if it is in the range [0,n) otherwise it returns n-1.
func (r constRand) Intn(n int) int {
	if n > int(r) {
		return int(r)
	}
	return n - 1
}

func TestFisherYatesPermute(t *testing.T) {
	tests := []struct {
		n         uint32
		rng       constRand
		nPermute  uint32
		want      []int
		wantState *FisherYates
	}{
		{
			n:        8,
			nPermute: 0,
			want:     []int{},
			wantState: &FisherYates{
				perm: []int{0, 1, 2, 3, 4, 5, 6, 7},
			},
		},
		{
			n:        8,
			nPermute: 3,
			rng:      0, // always "swaps" with self
			want:     []int{0, 1, 2},
			wantState: &FisherYates{
				perm:     []int{0, 1, 2, 3, 4, 5, 6, 7},
				shuffled: 3,
			},
		},
		{
			n:        4,
			nPermute: 4,
			rng:      0, // always "swaps" with self
			want:     []int{0, 1, 2, 3},
			wantState: &FisherYates{
				perm:     []int{0, 1, 2, 3},
				shuffled: 4,
			},
		},
		{
			n:        8,
			nPermute: 4,
			rng:      1, // always swaps with next
			want:     []int{1, 2, 3, 4},
			wantState: &FisherYates{
				perm:     []int{1, 2, 3, 4, 0, 5, 6, 7},
				shuffled: 4,
			},
		},
		{
			n:        8,
			nPermute: 8,
			rng:      1, // always swaps with next if available
			want:     []int{1, 2, 3, 4, 5, 6, 7, 0},
			wantState: &FisherYates{
				perm:     []int{1, 2, 3, 4, 5, 6, 7, 0},
				shuffled: 8,
			},
		},
		{
			n:        5,
			nPermute: 1,
			rng:      2,
			want:     []int{2},
			wantState: &FisherYates{
				perm:     []int{2, 1, 0, 3, 4}, // swaps 0 & 2
				shuffled: 1,
			},
		},
		{
			n:        5,
			nPermute: 2,
			rng:      2,
			want:     []int{2, 3},
			wantState: &FisherYates{
				perm:     []int{2, 3, 0, 1, 4}, // as above, then swaps 1 & 3
				shuffled: 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Logf("Running NewFisherYates(%d).Permute(%d, %T=%d)", tt.n, tt.nPermute, tt.rng, tt.rng)

			f := NewFisherYates(tt.n)

			got, err := f.Permute(tt.nPermute, tt.rng)
			if err != nil {
				t.Fatalf("Permute(…) error %v", err)
			}
			if diff := cmp.Diff(tt.want, got, diffOpts()); diff != "" {
				t.Errorf("Permute(…) diff (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.want, f.Permutation(), diffOpts()); diff != "" {
				t.Errorf("After Permute(…); Permutation() diff (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantState, f, diffOpts()); diff != "" {
				t.Errorf("After Permute(…) %T state diff (-want +got):\n%s", f, diff)
			}
		})
	}
}

func TestFisherYatesEndToEnd(t *testing.T) {
	const startSize = 3

	// Note that every test receives the same *FisherYates so they are
	// deliberately non-hermetic.
	tests := []struct {
		name            string
		action          func(*FisherYates)
		wantPermutation []int
		wantState       *FisherYates
	}{
		{
			name:   "initial state",
			action: func(*FisherYates) {},
			wantState: &FisherYates{
				perm:     []int{0, 1, 2},
				shuffled: 0,
			},
			wantPermutation: []int{},
		},
		{
			name: "GrowTo(smaller) is noop",
			action: func(f *FisherYates) {
				f.GrowTo(2) // already larger
			},
			wantState: &FisherYates{
				perm:     []int{0, 1, 2},
				shuffled: 0,
			},
			wantPermutation: []int{},
		},
		{
			name: "GrowTo(larger)",
			action: func(f *FisherYates) {
				f.GrowTo(5)
			},
			wantState: &FisherYates{
				perm:     []int{0, 1, 2, 3, 4},
				shuffled: 0,
			},
			wantPermutation: []int{},
		},
		{
			name: "partial shuffle",
			action: func(f *FisherYates) {
				f.mustPermute(t, 2, constRand(3))
			},
			wantState: &FisherYates{
				//  0  1  2  3  4
				// [0] 1  2 [3] 4
				// [3] 1  2 [0] 4
				//  3  1  2  0  4
				//
				//  3 [1] 2  0 [4]
				//  3 [4] 2  0 [1]
				//  3  4  2  0  1
				perm:     []int{3, 4, 2, 0, 1},
				shuffled: 2,
			},
			wantPermutation: []int{3, 4},
		},
		{
			name: "grow after partial shuffle",
			action: func(f *FisherYates) {
				f.Grow(5)
			},
			wantState: &FisherYates{
				perm:     []int{3, 4, 2, 0, 1, 5, 6, 7, 8, 9},
				shuffled: 2,
			},
			wantPermutation: []int{3, 4},
		},
		{
			name: "further shuffle after growth",
			action: func(f *FisherYates) {
				f.mustPermute(t, 1, constRand(5))
			},
			wantState: &FisherYates{
				//  3  4  2  0  1  5  6  7  8  9
				//  3  4 [2] 0  1  5  6 [7] 8  9
				//  3  4 [7] 0  1  5  6 [2] 8  9
				perm:     []int{3, 4, 7, 0, 1, 5, 6, 2, 8, 9},
				shuffled: 3,
			},
			wantPermutation: []int{3, 4, 7},
		},
		{
			name: "finish shuffling with no-op swaps",
			action: func(f *FisherYates) {
				f.PermuteUpTo(math.MaxUint32, constRand(0))
			},
			wantState: &FisherYates{
				perm:     []int{3, 4, 7, 0, 1, 5, 6, 2, 8, 9},
				shuffled: 10,
			},
			wantPermutation: []int{3, 4, 7, 0, 1, 5, 6, 2, 8, 9},
		},
		{
			name: "grow and shuffle after finished",
			action: func(f *FisherYates) {
				f.Grow(3)
				f.mustPermute(t, 1, constRand(1)) // swaps first two new indices, thus choosing the second
			},
			wantState: &FisherYates{
				perm:     []int{3, 4, 7, 0, 1, 5, 6, 2, 8, 9, 11, 10, 12},
				shuffled: 11,
			},
			wantPermutation: []int{3, 4, 7, 0, 1, 5, 6, 2, 8, 9, 11},
		},
	}

	f := NewFisherYates(startSize)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.action(f)

			if diff := cmp.Diff(tt.wantPermutation, f.Permutation(), diffOpts()); diff != "" {
				t.Errorf("%T.Permutation() diff (-want +got):\n%s", f, diff)
			}
			if diff := cmp.Diff(tt.wantState, f, diffOpts()); diff != "" {
				t.Fatalf("%T state diff (-want +got):\n%s", f, diff)
			}
		})
	}
}

// Is implements the interface allowing for use of errors.Is().
func (p *permuteTooMany) Is(q error) bool {
	switch q := q.(type) {
	case *permuteTooMany:
		return p.n == q.n && p.remain == q.remain
	default:
		return false
	}
}

func TestFisherYatesProperties(t *testing.T) {
	var anyValueInOriginalPosition bool

	for seed := int64(0); seed < 10; seed++ {
		const n = 100
		f := NewFisherYates(n)
		rng := rand.New(rand.NewSource(seed))

		var want []int
		for i := 0; i < n; i++ {
			want = append(want, f.mustPermute(t, 1, rng)...)
			if diff := cmp.Diff(want, f.Permutation()); diff != "" {
				t.Fatalf("%T.Permutation() after %d calls to Permute() expecting concatenation of Permute() return values diff (-want +got):\n%s", f, i+1, diff)
			}
			if got, want := f.shuffled, i+1; got != want {
				t.Errorf("%T.shuffled == %d; want %d (number of calls to Permute(1))", f, got, want)
			}
		}

		t.Run("Permute too many", func(t *testing.T) {
			grow := uint32(rng.Intn(10))
			n := grow + uint32(rng.Intn(10)) + 1
			f.Grow(grow)

			wantErr := &permuteTooMany{n: n, remain: grow}
			if _, err := f.Permute(n, rng); !errors.Is(err, wantErr) {
				t.Errorf("After all values permuted then call to %T.Grow(%d), call to %T.Permute(%d) got error %v; want %T(%+v)", f, grow, f, n, err, wantErr, wantErr)
			}
		})

		for i, val := range f.Permutation() {
			anyValueInOriginalPosition = anyValueInOriginalPosition || i == val
			// This test only requires a single example of a value remaining in its
			// original position to demonstrate that we haven't accidentally got
			// Sattolo's algorithm.
			if i == val {
				return
			}
		}
	}

	if !anyValueInOriginalPosition {
		t.Error("No values were in the same position after full permutation")
	}
}
