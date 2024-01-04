// Binary holder fetches holders balances for a set of input ERC721 collections.
// It reads new-line delimited contract addresses from stdin and writes
// token balances as CSV to stdout (row = holder, column = collection).
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"os"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/sync/errgroup"

	"github.com/cxkoda/solgo/contracts/erc"
	"github.com/cxkoda/solgo/go/eth"
	"github.com/cxkoda/solgo/go/proof"
)

func main() {
	d := eth.MustNewDialerFromFlag(flag.CommandLine, proof.InfuraMainnetURL())
	flag.Parse()
	if err := run(context.Background(), d, os.Stdin, os.Stdout); err != nil {
		exit(err)
	}
}

// exit prints err to stderr and exits with code 1.
func exit(err error) {
	fmt.Fprint(os.Stderr, err)
	os.Exit(1)
}

// run reads new-line delimeted address from addrSrc, finds all of their
// delegations, and writes a CSV to out.
func run(ctx context.Context, d *eth.Dialer, addrSrc io.Reader, out io.Writer) error {
	client, err := d.Dial(ctx)
	if err != nil {
		return fmt.Errorf("%T.Dial(): %v", d, err)
	}
	defer client.Close()

	addrs, err := eth.AddressPerLine(addrSrc)
	if err != nil {
		return fmt.Errorf("eth.AddressPerLine((…): %v", err)
	}

	balances := make(map[common.Address]map[common.Address]uint64)

	for _, tokenAddress := range addrs {
		token, err := erc.NewIERC721Enumerable(tokenAddress, client)
		if err != nil {
			return fmt.Errorf("erc.NewIERC721Enumerable(…): %v", err)
		}

		g, ctx := errgroup.WithContext(ctx)

		var mu sync.Mutex
		done := new(uint64)
		supply, err := token.TotalSupply(nil)
		if err != nil {
			return fmt.Errorf("%T.TotalSupply(): %v", token, err)
		}

		n := supply.Int64()
		for i := int64(0); i < n; i++ {
			tokenId := new(big.Int).SetInt64(i)
			g.Go(func() error {
				owner, err := token.OwnerOf(&bind.CallOpts{Context: ctx}, tokenId)
				if err != nil {
					return err
				}

				mu.Lock()
				defer mu.Unlock()

				if balances[owner] == nil {
					balances[owner] = make(map[common.Address]uint64)
				}
				balances[owner][tokenAddress]++

				atomic.AddUint64(done, 1)
				log.Printf("%d/%d (at least)", *done, n)
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}
		log.Printf("%d/%d", *done, n)
	}

	return writeCSV(out, addrs, balances)
}

// writeCSV writes a CSV containing holder addresses and token balances as rows and collections as columns.
func writeCSV(w io.Writer, tokenAddrs []common.Address, balances map[common.Address]map[common.Address]uint64) error {
	var rows [][]string
	row := []string{"address"}
	for _, a := range tokenAddrs {
		row = append(row, a.String())
	}
	rows = append(rows, row)

	for h, bs := range balances {
		row := []string{h.String()}
		for _, a := range tokenAddrs {
			row = append(row, fmt.Sprintf("%d", bs[a]))
		}
		rows = append(rows, row)
	}

	return csv.NewWriter(w).WriteAll(rows)
}
