package eth_test

import (
	"context"
	"flag"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cxkoda/solgo/go/ethtest"
	"github.com/cxkoda/solgo/go/secrets"

	// See eth_test.go for rationale behind a dot import. This MUST NOT be
	// considered precedent outside of tests and SHOULD be avoided where
	// possible.
	. "github.com/cxkoda/solgo/go/eth"
)

func TestDialerFromFlag(t *testing.T) {
	ctx := context.Background()

	const (
		chain0 = 42 << 42
		chain1 = 1337
	)

	urls := make(map[uint64]*secrets.Secret)
	for _, id := range []uint64{chain0, chain1} {
		urls[id] = &secrets.Secret{
			Source: secrets.Raw,
			ID:     ethtest.NewRPCStub(id, 0).ServeHTTP(t),
		}
	}

	tests := []struct {
		defaultURL, overrideURL *secrets.Secret
		want                    uint64
	}{
		{
			defaultURL:  urls[chain0],
			overrideURL: urls[chain1],
			want:        chain1,
		},
		{
			defaultURL:  urls[chain1],
			overrideURL: urls[chain0],
			want:        chain0,
		},
		{
			defaultURL:  nil,
			overrideURL: urls[chain0],
			want:        chain0,
		},
		{
			defaultURL:  nil,
			overrideURL: urls[chain1],
			want:        chain1,
		},
		{
			defaultURL:  urls[chain0],
			overrideURL: nil,
			want:        chain0,
		},
		{
			defaultURL:  urls[chain1],
			overrideURL: nil,
			want:        chain1,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)

			dialer, err := NewDialerFromFlag(fs, tt.defaultURL)
			if err != nil {
				t.Fatalf("NewDialerFromFlag(%v, %+v) error %v", fs, tt.defaultURL, err)
			}

			{ // not using t.Run as t.Fatal must bail on the entire test.
				f := fs.Lookup(DialerFlag)
				if f == nil {
					t.Fatalf("%T.Lookup(%q) default value after NewDialerFromFlag(…) got nil; want non-nil", fs, DialerFlag)
				}
				got := f.DefValue

				want := (&secrets.Secret{}).String()
				if tt.defaultURL != nil {
					want = tt.defaultURL.String()
				}
				if got != want {
					t.Fatalf("%T.Lookup(%q) default value after NewDialerFromFlag(…, %+v); got %q; want %q", fs, DialerFlag, tt.defaultURL, got, want)
				}
			}

			if tt.overrideURL != nil {
				if err := fs.Set(DialerFlag, tt.overrideURL.String()); err != nil {
					t.Fatalf("%T.Set(%q, %q) error %v", fs, DialerFlag, tt.overrideURL, err)
				}
			}
			if err := fs.Parse(nil); err != nil {
				t.Fatalf("%T.Parse(nil) error %v", fs, err)
			}

			client, err := dialer.Dial(ctx)
			if err != nil {
				t.Fatalf("%T.Dial(ctx) error %v", dialer, err)
			}
			if got, err := client.ChainID(ctx); err != nil || got.Cmp(new(big.Int).SetUint64(tt.want)) != 0 {
				t.Errorf("%T from %T.Dial(); ChainID() got %d, err = %v; want %d, nil err", client, dialer, got, err, tt.want)
			}
		})
	}
}

func TestDemux(t *testing.T) {
	rSim := ethtest.NewSimulatedBackendTB(t, 1)
	wSim := ethtest.NewSimulatedBackendTB(t, 1)

	deploy := func(t *testing.T, sim *ethtest.SimulatedBackend, payload string) (common.Address, *RW) {
		t.Helper()
		addr, _, c, err := DeployRW(sim.Acc(0), sim)
		if err != nil {
			t.Fatalf("DeployRW(…) error %v", err)
		}
		sim.Must(t, "%T.Write(…, %q)", c, payload)(c.Write(sim.Acc(0), payload))
		return addr, c
	}

	rAddr, rBind := deploy(t, rSim, "r")
	wAddr, wBind := deploy(t, wSim, "w")
	if rAddr != wAddr {
		t.Fatalf("Different addresses for read/write contracts; got read = %v, write = %v", rAddr, wAddr)
	}
	addr := rAddr

	demuxSim := RWDemuxBackend(rSim, wSim)
	demux, err := NewRW(addr, demuxSim)
	if err != nil {
		t.Fatalf("NewRW([deployed contract], %T) error %v", demuxSim, err)
	}

	t.Run("read", func(t *testing.T) {
		tests := []struct {
			name    string
			binding *RW
			want    string
		}{
			{
				name:    "direct read binding",
				binding: rBind,
				want:    "r",
			},
			{
				name:    "direct write binding",
				binding: wBind,
				want:    "w",
			},
			{
				name:    "demux binding",
				binding: demux,
				want:    "r",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got, err := tt.binding.Read(nil); err != nil || got != tt.want {
					t.Errorf("%T.Read(nil) got %q, err = %v; want %q, nil err", demux, got, err, tt.want)
				}
			})
		}
	})

	wSim.Must(t, "demuxed %T.Write(…)", demux)(demux.Write(wSim.Acc(0), "updated"))

	t.Run("read after write", func(t *testing.T) {
		tests := []struct {
			name    string
			binding *RW
			want    string
		}{
			{
				name:    "direct read binding",
				binding: rBind,
				want:    "r",
			},
			{
				name:    "direct write binding",
				binding: wBind,
				want:    "updated",
			},
			{
				name:    "demux binding",
				binding: demux,
				want:    "r",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got, err := tt.binding.Read(nil); err != nil || got != tt.want {
					t.Errorf("%T.Read(nil) got %q, err = %v; want %q, nil err", demux, got, err, tt.want)
				}
			})
		}
	})
}
