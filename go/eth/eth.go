// Package eth provides general functionality for interacting with the Ethereum
// network, smart contracts, etc.
package eth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"math/rand"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/cxkoda/solgo/go/memconv"
)

// Symbol is the ETH symbol.
const Symbol = `Ξ`

// AddressPerLine returns AddressesFromReader(r, bufio.ScanLines). It therefore
// reads each non-empty line of r and parses it as an Address. Empty lines are
// skipped.
func AddressPerLine(r io.Reader) ([]common.Address, error) {
	return AddressesFromReader(r, bufio.ScanLines)
}

// AddressesFromReader scans the Reader, splitting according to the split
// function, and parses each non-empty chunk as an address after trimming
// surrounding space (i.e. the split function does not have to trim). Empty
// chunks are skipped.
func AddressesFromReader(r io.Reader, split bufio.SplitFunc) ([]common.Address, error) {
	s := bufio.NewScanner(r)
	s.Split(split)

	var addrs []common.Address

	for s.Scan() {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			continue
		}
		if !common.IsHexAddress(raw) {
			return nil, fmt.Errorf("invalid address %q", raw)
		}
		addrs = append(addrs, common.HexToAddress(raw))
	}
	if err := s.Err(); err != nil {
		return nil, err
	}

	return addrs, nil
}

// A BlockFetcher can fetch information about blocks. Typically this would be an
// *ethclient.Client.
type BlockFetcher interface {
	BlockNumber(context.Context) (uint64, error)
	BlockByNumber(context.Context, *big.Int) (*types.Block, error)
}

// ErrBlockNotFound is returned by LastBlockBy if no block was mined by the
// requested time.
var ErrBlockNotFound = errors.New("block not found")

// BlockRange is an inclusive range of block numbers.
type BlockRange struct {
	First, Last uint64
}

// LastBlockBy performs a binary search to find and return the last block mined
// by the specified unix timestamp, inclusive. If a nil hint is provided, the
// search defaults to [0,blocks.BlockNumber()]. If hint.Last == 0, it defaults
// to the latest block.
func LastBlockBy(ctx context.Context, blocks BlockFetcher, minedBy uint64, hint *BlockRange) (_ *types.Block, retErr error) {
	if hint == nil {
		hint = &BlockRange{}
	} else {
		cp := *hint
		hint = &cp
	}
	if hint.Last == 0 {
		curr, err := blocks.BlockNumber(ctx)
		if err != nil {
			return nil, fmt.Errorf("%T.BlockNumber(): %v", blocks, err)
		}
		if curr >= math.MaxInt { // i.e. can int represent curr+1?
			return nil, fmt.Errorf("block %d too large for binary search", curr)
		}
		hint.Last = curr
	}

	// The stdlib binary search doesn't support errors so we have to panic to
	// break early.
	defer func() {
		if r := recover(); retErr == nil && r != nil {
			retErr = fmt.Errorf("%v", r)
		}
	}()

	// From its documentation: Search uses binary search to find and return the
	// smallest index i in [0, n) at which f(i) is true.
	//
	// We must therefore search on [0,curr+1) to include the current block,
	// return false if the block was mined by `minedBy`, and subtract one from
	// the returned value.
	var result *types.Block
	firstMinedAfter := sort.Search(int(hint.Last-hint.First+1), func(num int) bool {
		num += int(hint.First)
		b, err := blocks.BlockByNumber(ctx, big.NewInt(int64(num)))
		if err != nil {
			panic(fmt.Errorf("%T.BlockByNumber(%d): %v", blocks, num, err))
		}

		mined := b.Time() <= minedBy
		if mined && (result == nil || result.Time() < b.Time()) {
			result = b
		}
		return !mined
	})
	if result == nil {
		return nil, ErrBlockNotFound
	}
	firstMinedAfter += int(hint.First)

	if got := result.NumberU64(); got != uint64(firstMinedAfter-1) {
		// If this happens there's a bug. Although we don't technically need to
		// do the check, it's cheap and acts as an internal sense check.
		return nil, fmt.Errorf("broken invariant: sort.Search() found first block mined after %v as %d but %T.NumberU64() = %d (should be 1 before)", minedBy, firstMinedAfter, result, got)
	}

	return result, nil
}

// RandFromHash returns a new *rand.Rand seeded by the Hash. The Hash is
// converted to a uint256 and each of its four uint64 words are folded into one
// by xor. The raw bits of the resulting uint64 are treated as an int64 passed
// to rand.NewSource().
func RandFromHash(h common.Hash) *rand.Rand {
	u := *uint256.MustFromBig(h.Big()) // h.Big() can't overflow a uint256
	seed := u[0] ^ u[1] ^ u[2] ^ u[3]
	src := rand.NewSource(memconv.Cast[uint64, int64](&seed))
	return rand.New(src)
}
