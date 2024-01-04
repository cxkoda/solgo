package main

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"text/template"

	"github.com/gocarina/gocsv"
	"github.com/google/go-cmp/cmp"

	"github.com/cxkoda/solgo/contracts/delegate"
	impl "github.com/cxkoda/solgo/contracts/delegate/delegateimpl"
	"github.com/cxkoda/solgo/go/ethtest"
)

const (
	deployer = iota
	vault
	delegatedAll
	delegatedContract
	contract
	delegatedToken
	tokenContract
	numAccounts
)

func TestCSVRoundTrip(t *testing.T) {
	ctx := context.Background()
	sim := ethtest.NewSimulatedBackendTB(t, numAccounts)

	addr, _, _, err := impl.DeployDelegationRegistry(sim.Acc(deployer), sim)
	if err != nil {
		t.Fatalf("DeployDelegationRegistry(…) error %v", err)
	}

	reg, err := delegate.NewIDelegationRegistry(addr, sim)
	if err != nil {
		t.Fatalf("NewIDelegationRegistry([address just deployed to %T]) error %v", sim, err)
	}

	const tokenID = 42
	sim.Must(t, "%T.DelegateForAll()", reg)(reg.DelegateForAll(sim.Acc(vault), sim.Addr(delegatedAll), true))
	sim.Must(t, "%T.DelegateForContract()", reg)(reg.DelegateForContract(sim.Acc(vault), sim.Addr(delegatedContract), sim.Addr(contract), true))
	sim.Must(t, "%T.DelegateForToken()", reg)(reg.DelegateForToken(sim.Acc(vault), sim.Addr(delegatedToken), sim.Addr(tokenContract), big.NewInt(tokenID), true))

	var addrs []string
	for i := 0; i < numAccounts; i++ {
		addrs = append(addrs, sim.Addr(i).Hex())
	}
	addrSrc := strings.NewReader(strings.Join(addrs, "\n"))

	gotCSV := new(bytes.Buffer)
	if err := fetchDelegationsAndExportCSV(ctx, reg, addrSrc, gotCSV); err != nil {
		t.Fatalf("fetchDelegationsAndExportCSV(…) error %v", err)
	}

	t.Run("CSV", func(t *testing.T) {
		wantTmpl := template.Must(template.New("want").Parse(`Vault,Delegate,Contract,TokenID
{{.vault}},{{.delegatedAll}},,
{{.vault}},{{.delegatedContract}},{{.contract}},
{{.vault}},{{.delegatedToken}},{{.tokenContract}},{{.tokenID}}
`))

		data := map[string]string{
			"vault":             sim.Addr(vault).Hex(),
			"delegatedAll":      sim.Addr(delegatedAll).Hex(),
			"delegatedContract": sim.Addr(delegatedContract).Hex(),
			"contract":          sim.Addr(contract).Hex(),
			"delegatedToken":    sim.Addr(delegatedToken).Hex(),
			"tokenContract":     sim.Addr(tokenContract).Hex(),
			"tokenID":           fmt.Sprintf("%#x", tokenID),
		}

		want := new(bytes.Buffer)
		if err := wantTmpl.Execute(want, data); err != nil {
			t.Fatalf("Building expected CSV output; %T.Execute(…) error %v", wantTmpl, err)
		}

		// Ignore checksum vs regular address differences.
		opt := cmp.Transformer("lowercase", strings.ToLower)

		if diff := cmp.Diff(want.String(), gotCSV.String(), opt); diff != "" {
			// Printing full values as well as the diff because the diff can be
			// confusing as it tries to split by columns too, not just lines.
			t.Errorf("fetchDelegationsAndExportCSV(…) wrote CSV:\n\n%s\nwant:\n\n%s\n\ndiff (-want +got):\n%s", gotCSV, want, diff)
		}
	})

	if t.Failed() {
		// We know that `got` is invalid so no point trying to reverse it.
		return
	}

	t.Run("unmarshal CSV", func(t *testing.T) {
		var got []*delegate.Delegation
		if err := gocsv.Unmarshal(gotCSV, &got); err != nil {
			t.Fatalf("gocsv.Unmarshal([CSV written by fetchDelegationsAndExportCSV()], %T) error %v", &got, err)
		}
	})
}
