package contract

import (
	"fmt"
	"testing"

	"github.com/cxkoda/solgo/go/ethtest"
)

func TestContractEcho(t *testing.T) {
	sim := ethtest.NewSimulatedBackendTB(t, 1)
	_, _, demo, err := DeployDemo(sim.Acc(0), sim)
	if err != nil {
		t.Fatalf("DeployDemo() error %v", err)
	}

	for _, input := range []string{
		"Hello",
		"Hello world",
		"",
	} {
		got, err := demo.Echo(nil, input)
		if err != nil {
			t.Errorf("%T.Echo(nil, %q) error %v", demo, input, err)
			continue
		}
		if want := fmt.Sprintf("Solidity: %s", input); got != want {
			t.Errorf("%T.Echo(nil, %q) got %q; want %q", demo, input, got, want)
		}
	}
}
