package ethtest

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/h-fam/errdiff"
)

func TestRPCStub(t *testing.T) {
	ctx := context.Background()

	const chainID = 42
	stub := NewRPCStub(chainID, 0)

	client, err := ethclient.Dial(stub.ServeHTTP(t))
	if err != nil {
		t.Fatalf("ethclient.Dial(%T.ServeHTTP()) error %v", stub, err)
	}
	defer client.Close()

	t.Logf("All methods called on a %T dialling the address returned by %T.ServeHTTP()", client, stub)

	if got, err := client.ChainID(ctx); err != nil || !got.IsUint64() || got.Uint64() != chainID {
		t.Errorf("ChainID() got %v, err = %v; want %d, nil err", got, err, chainID)
	}

	t.Run("BlockNumber", func(t *testing.T) {
		for i := uint64(0); i < 10; i++ {
			stub.SetBlockNumber(i)

			if got, err := client.BlockNumber(ctx); err != nil || got != i {
				t.Errorf("BlockNumber() got %d, err = %v; want %d, nil err", got, err, i)
			}
		}
	})

	t.Run("GetBlockByNumber valid blocks", func(t *testing.T) {
		for i := uint64(0); i < 10; i++ {
			stub.SetBlockNumber(i)

			wantNumber := i

			// Block's headers are unexported so we can't compare the whole data structure:
			// https://github.com/ethereum/go-ethereum/blob/v1.12.0/core/types/block.go#L175.
			got, err := client.BlockByNumber(ctx, new(big.Int).SetUint64(wantNumber))
			if err != nil {
				t.Fatalf("BlockByNumber() error %v", err)
			}
			if got.NumberU64() != wantNumber {
				t.Errorf("BlockByNumber() Number got %d, want %d", got.NumberU64(), wantNumber)
			}
		}
	})

	t.Run("GetBlockByNumber invalid blocks ahead of current block", func(t *testing.T) {
		for i := uint64(0); i < 10; i++ {
			stub.SetBlockNumber(i)

			wantNumber := i + 10

			_, err := client.BlockByNumber(ctx, new(big.Int).SetUint64(wantNumber))
			if diff := errdiff.Substring(err, fmt.Sprint(wantNumber)); diff != "" {
				t.Errorf("BlockByNumber() %s", diff)
			}
		}
	})
}
