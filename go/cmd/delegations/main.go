// Package delegations finds all delegate.cash delegations for a set of input
// addresses. It reads new-line delimited addresses from stdin and writes a CSV
// of delegations (of all types) to stdout.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"sync/atomic"

	"github.com/gocarina/gocsv"
	"golang.org/x/sync/errgroup"

	"github.com/proofxyz/solgo/contracts/delegate"
	"github.com/proofxyz/solgo/go/eth"
	"github.com/proofxyz/solgo/go/proof"
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

	reg, err := delegate.New(client)
	if err != nil {
		return fmt.Errorf("delegate.New(…): %v", err)
	}
	return fetchDelegationsAndExportCSV(ctx, reg, addrSrc, out)
}

func fetchDelegationsAndExportCSV(ctx context.Context, reg *delegate.IDelegationRegistry, addrSrc io.Reader, out io.Writer) error {
	vaults, err := eth.AddressPerLine(addrSrc)
	if err != nil {
		return err
	}

	var vaultDelegates []*delegate.Delegation
	var mu sync.Mutex
	// For progress logging.
	n := len(vaults)
	done := new(uint64)

	g, ctx := errgroup.WithContext(ctx)
	for _, v := range vaults {
		vault := v
		g.Go(func() error {
			var delegations []*delegate.Delegation

			delegations, err := delegate.AppendDelegations(ctx, delegations, reg.GetAppendableDelegatesForAll, vault)
			if err != nil {
				return err
			}

			delegations, err = delegate.AppendDelegations(ctx, delegations, reg.GetContractLevelDelegations, vault)
			if err != nil {
				return err
			}

			delegations, err = delegate.AppendDelegations(ctx, delegations, reg.GetTokenLevelDelegations, vault)
			if err != nil {
				return err
			}

			atomic.AddUint64(done, 1)
			log.Printf("%d/%d (at least)", atomic.LoadUint64(done), n)

			mu.Lock()
			defer mu.Unlock()
			vaultDelegates = append(vaultDelegates, delegations...)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	log.Printf("%d/%d", *done, n)

	return gocsv.Marshal(vaultDelegates, out)
}
