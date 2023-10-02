package eth_test

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/go-cmp/cmp"
	"github.com/h-fam/errdiff"

	"github.com/proofxyz/solgo/go/ethtest"

	// These tests require ethtest.SimulatedBackend but that would result in a
	// cyclical dependency. As this is limited to these tests and not the
	// package itself, we simply move them from package eth to package eth_test
	// and dot-import to avoid having to qualify the full package name. This
	// MUST NOT be considered precedent outside of tests and SHOULD be avoided
	// where possible.
	. "github.com/proofxyz/solgo/go/eth"
)

func TestAddressPerLine(t *testing.T) {
	const (
		addr0 = "0x0123456789012345678901234567890123456789"
		addr1 = "0xabcdef0000abcdef0000abcdef0000abcdef0000"
	)

	tests := []struct {
		name, input    string
		want           []common.Address
		errDiffAgainst interface{}
	}{
		{
			name: "empty input",
		},
		{
			name: "only whitespace",
			input: `		

		
   `,
		},
		{
			name:           "invalid address",
			input:          `	hello	`,
			errDiffAgainst: "hello",
		},
		{
			name: "simple happy path",
			input: `		` + addr0 + `
	
	` + addr1 + `  
	   `,
			want: []common.Address{
				common.HexToAddress(addr0),
				common.HexToAddress(addr1),
			},
		},
		{
			name: "fuzzed",
			// input added below
		},
	}

	fuzz := &tests[len(tests)-1]

	s := crypto.NewKeccakState()
	curve := crypto.S256()
	for i := 0; i < 10; i++ {
		key, err := ecdsa.GenerateKey(curve, s)
		if err != nil {
			t.Fatalf("ecdsa.GenerateKey(%T, %T) error %T", curve, s, err)
		}

		addr := crypto.PubkeyToAddress(key.PublicKey)
		fuzz.input += fmt.Sprintf("\n %v\t  \t", addr)
		fuzz.want = append(fuzz.want, addr)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AddressPerLine(strings.NewReader(tt.input))
			if diff := errdiff.Check(err, tt.errDiffAgainst); diff != "" {
				t.Fatalf("AddressPerLine(%q) %s", tt.input, diff)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("AddressPerLine(%q) diff (-want +got):\n%s", tt.input, diff)
			}
		})
	}
}

// The BlockFetcher documentation states that it usually expects an
// ethclient.Client, so lock that in.
var _ BlockFetcher = &ethclient.Client{}

func TestLastBlockBy(t *testing.T) {
	ctx := context.Background()

	type blockTimes = ethtest.BlockTimes
	newBlock := ethtest.NewBlock

	type test struct {
		name    string
		blocks  blockTimes
		minedBy uint64
		hint    *BlockRange
		want    *types.Block
		wantErr error
	}

	tests := []test{
		{
			name:    "before genesis block",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 2,
			wantErr: ErrBlockNotFound,
		},
		{
			name:    "genesis block",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 3,
			want:    newBlock(0, 3),
		},
		{
			name:    "exact block time",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 11,
			want:    newBlock(2, 11),
		},
		{
			name:    "exact block time minus 1",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 10,
			want:    newBlock(1, 8),
		},
		{
			name:    "exact block time plus 1",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 12,
			want:    newBlock(2, 11),
		},
		{
			name:    "sandwiched block",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 17,
			want:    newBlock(4, 17),
		},
		{
			name:    "one before sandwiched block",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 16,
			want:    newBlock(3, 16),
		},
		{
			name:    "one after sandwiched block",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 18,
			want:    newBlock(5, 18),
		},
		{
			name:    "exactly current block",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 20,
			want:    newBlock(6, 20),
		},
		{
			name:    "second after current block",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 21,
			want:    newBlock(6, 20),
		},
		{
			name:    "far in the future",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: math.MaxUint64,
			want:    newBlock(6, 20),
		},
		{
			name:    "valid hint",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 16,
			hint: &BlockRange{
				First: 2,
				Last:  4,
			},
			want: newBlock(3, 16),
		},
		{
			name:    "valid hint",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 8,
			hint: &BlockRange{
				First: 0,
				Last:  1,
			},
			want: newBlock(1, 8),
		},
		{
			name:    "full-range hint",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 16,
			hint: &BlockRange{
				First: 0,
				Last:  6,
			},
			want: newBlock(3, 16),
		},
		{
			name:    "exact hint",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 16,
			hint: &BlockRange{
				First: 3,
				Last:  3,
			},
			want: newBlock(3, 16),
		},
		{
			name:    "invalid hint",
			blocks:  blockTimes{3, 8, 11, 16, 17, 18, 20},
			minedBy: 11,
			hint: &BlockRange{
				First: 3,
				Last:  3,
			},
			wantErr: ErrBlockNotFound,
		},
	}

	{
		var (
			fuzz blockTimes
			last uint64
		)
		rng := rand.New(rand.NewSource(42))
		for i := 0; i < 20; i++ {
			last += uint64(rng.Intn(5)) + 1
			fuzz = append(fuzz, last)
		}

		block := -1
		for by := uint64(0); by <= last+1; by++ {
			if block+1 < len(fuzz) && by == fuzz[block+1] {
				block++
			}

			tt := test{
				name:    "fuzz",
				blocks:  fuzz,
				minedBy: by,
			}
			if block == -1 {
				tt.wantErr = ErrBlockNotFound
			} else {
				tt.want = newBlock(int64(block), fuzz[block])
			}

			tests = append(tests, tt)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 1; i < len(tt.blocks); i++ {
				if tt.blocks[i-1] >= tt.blocks[i] {
					t.Fatalf("bad test setup; blocks[%d] >= blocks[%d] implying mined out of order", i-1, i)
				}
			}

			got, err := LastBlockBy(ctx, tt.blocks, tt.minedBy, tt.hint)
			if err != tt.wantErr {
				t.Fatalf("LastBlockBy(%d, %d, %+v) error = %v, want %v", tt.blocks, tt.minedBy, tt.hint, err, tt.wantErr)
			}
			if err != nil || tt.wantErr != nil {
				return
			}

			opt := cmp.Comparer(func(a, b *types.Block) bool {
				if a == nil || b == nil {
					return false
				}
				return a.Number().Cmp(b.Number()) == 0 && a.Time() == b.Time()
			})
			if diff := cmp.Diff(tt.want, got, opt); diff != "" {
				t.Errorf("LastBlockBy(%d, %d, %+v) (-want +got):\n%s", tt.blocks, tt.minedBy, tt.hint, diff)
			}
		})
	}
}

func TestLockRandFromHash(t *testing.T) {
	want := map[string][]int{
		"hello": {6567006441452633194, 4804437169785775181, 214950417650711817},
		"world": {2343618045756208216, 1005978362200562152, 1846823089990389684},
		"foo":   {5535734994961986113, 6935999750324427497, 2984914241301402157},
		"bar":   {8890929298216866351, 4363264265745787575, 2988506460283127108},
	}

	got := map[string][]int{}

	for k := range want {
		rng := RandFromHash(crypto.Keccak256Hash([]byte(k)))

		var vals []int
		for i := 0; i < len(want[k]); i++ {
			vals = append(vals, rng.Int())
		}
		got[k] = vals
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("RandFromHash() produces different outputs; hashing map keys then calling rng.Int(); diff (-want +got):\n%s", diff)
	}
}
