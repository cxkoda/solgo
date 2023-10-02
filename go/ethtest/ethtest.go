// Package ethtest provides functionality for Ethereum testing.
package ethtest

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
)

// BlockTimes implements eth.BlockFetcher, returning Blocks with nothing but a
// mining time.
type BlockTimes []uint64

// BlockNumber returns len(t)-1.
func (t BlockTimes) BlockNumber(ctx context.Context) (uint64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	n := uint64(len(t))
	if n == 0 {
		return 0, errors.New("no blocks mined")
	}
	return n - 1, nil
}

// BlockByNumber returns the requested block.
func (t BlockTimes) BlockByNumber(ctx context.Context, num *big.Int) (*types.Block, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if !num.IsInt64() {
		return nil, fmt.Errorf("block number %v is not int64", num)
	}

	i := int(num.Int64())
	if i > len(t) {
		return nil, fmt.Errorf("block %d not mined", i)
	}
	return NewBlock(num.Int64(), t[i]), nil
}

// NewBlock returns a new Block with only the number and time populated.
func NewBlock(num int64, time uint64) *types.Block {
	hdr := &types.Header{
		Number: big.NewInt(num),
		Time:   time,
	}
	return types.NewBlock(hdr, nil, nil, nil, nil)
}
