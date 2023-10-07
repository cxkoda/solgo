package main

import (
	"context"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/proofxyz/solgo/contracts/entropy"
	"github.com/proofxyz/solgo/go/eth"
	"github.com/proofxyz/solgo/go/ethtest"
)

const simBackendChainID = 1337

const (
	deployer = iota
	admin
	steerer
	public
	requester

	numAccounts
)

// newTestServer returns an httptest.Server handled by a signing source.
func newTestServer(t *testing.T, blockSrc blockSource, key entropySrc, chainID uint64) *httptest.Server {
	t.Helper()

	s, err := eth.DefaultHDPathPrefix.SignerFromPRF(key, nil, 0)
	if err != nil {
		t.Fatalf("%T(%v).SignerFromPRF(%T, nil, 0) error %v", eth.DefaultHDPathPrefix, eth.DefaultHDPathPrefix, key, err)
	}

	src, err := newSource(blockSrc, 0 /* blockInterval*/, s, chainID)
	if err != nil {
		t.Fatalf("newSource(…) error %v", err)
	}
	server := httptest.NewServer(src)
	t.Cleanup(server.Close)

	return server
}

func httpGet(t *testing.T, s *httptest.Server, path string) []byte {
	t.Helper()
	path = strings.TrimLeft(path, "/")
	url := fmt.Sprintf("%s/%s", s.URL, path)

	res, err := s.Client().Get(url)
	if err != nil || res.StatusCode != 200 {
		var code int
		if res != nil {
			code = res.StatusCode
		}
		t.Fatalf("%T.Get(%q): status %d; err = %v", s.Client(), url, code, err)
	}

	buf, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("io.ReadAll(%T.Body): %v", res, err)
	}
	return buf
}

func TestSourceSigning(t *testing.T) {
	const latestMinedBlock = 42
	blockSrc := func(context.Context) (uint64, error) { return latestMinedBlock, nil }
	realSrv := newTestServer(t, blockSrc, entropySrc("valid-private-key"), simBackendChainID)
	vandalSrv := newTestServer(t, blockSrc, entropySrc("other-private-key"), simBackendChainID)
	testnetSrv := newTestServer(t, blockSrc, entropySrc("valid-private-key"), simBackendChainID+1)

	signerAddr := common.HexToAddress(string(httpGet(t, realSrv, signerAddrEndpoint)))

	tests := []struct {
		name                              string
		signingServer                     *httptest.Server
		evmBlock, signBlock, provideBlock uint64
		wantErr                           bool
	}{
		{
			name:          "latest mined block",
			signingServer: realSrv,
			evmBlock:      latestMinedBlock,
			signBlock:     latestMinedBlock,
			provideBlock:  latestMinedBlock,
			wantErr:       false,
		},
		{
			name:          "latest mined block signed by different address",
			signingServer: vandalSrv,
			evmBlock:      latestMinedBlock,
			signBlock:     latestMinedBlock,
			provideBlock:  latestMinedBlock,
			wantErr:       true,
		},
		{
			name:          "latest mined block signed for different chain",
			signingServer: testnetSrv,
			evmBlock:      latestMinedBlock,
			signBlock:     latestMinedBlock,
			provideBlock:  latestMinedBlock,
			wantErr:       true,
		},
		{
			name:          "signature for incorrect block number",
			signingServer: realSrv,
			evmBlock:      latestMinedBlock,
			signBlock:     latestMinedBlock - 1,
			provideBlock:  latestMinedBlock,
			wantErr:       true,
		},
		{
			name:          "previously mined block",
			signingServer: realSrv,
			evmBlock:      latestMinedBlock,
			signBlock:     latestMinedBlock - 1,
			provideBlock:  latestMinedBlock - 1,
			wantErr:       false,
		},
		{
			name:          "future block",
			signingServer: realSrv,
			// The EVM block must be more than 1 behind because the request for
			// entropy will increment it.
			evmBlock:     latestMinedBlock - 5,
			signBlock:    latestMinedBlock,
			provideBlock: latestMinedBlock,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signBlock := new(big.Int).SetUint64(tt.signBlock)
			provideBlock := new(big.Int).SetUint64(tt.provideBlock)
			evmBlock := new(big.Int).SetUint64(tt.evmBlock)

			sim := ethtest.NewSimulatedBackendTB(t, numAccounts)
			_, _, oracle, err := entropy.DeployEntropyOracleV2(sim.Acc(0), sim, sim.Addr(admin), sim.Addr(steerer))
			if err != nil {
				t.Fatalf("DeployEntropyOracle(…): %v", err)
			}
			sim.Must(t, "%T.SetSigner([addr from %q])", oracle, signerAddrEndpoint)(oracle.SetSigner(sim.Acc(steerer), signerAddr))

			if !sim.FastForward(evmBlock) {
				t.Fatalf("%T.FastForward(%d) returned false", sim, evmBlock)
			}
			if got, want := sim.BlockNumber(), evmBlock; got.Cmp(want) != 0 {
				t.Fatalf("%T.BlockNumber() = %d; want %d", sim, got, evmBlock)
			}

			role, err := oracle.ENTROPYREQUESTERROLE(nil)
			if err != nil {
				t.Fatalf("%T.ENTROPYREQUESTERROLE(nil) error %v", oracle, err)
			}
			sim.Must(t, "%T.GrantRole(ENTROPY_REQUESTER_ROLE)", oracle)(oracle.GrantRole(sim.Acc(steerer), role, sim.Addr(requester)))

			// There are multiple concurrent actors in this test, an entropy
			// requester, a provider, and a final consumer. The provider and
			// consumer block while waiting for contract events in order to
			// mimic real-world flows. However event subscription may occur
			// after event emission, causing permanent blocking so here are some
			// subscriptions "we prepared earlier" ;)
			entropyRequests, entropyRequestErrs := watch(t, oracle.WatchEntropyRequested)
			entropyProvisions, entropyProvisionErrs := watch(t, oracle.WatchEntropyProvided)

			t.Run("requester", func(t *testing.T) {
				t.Parallel()
				sim.Must(t, "%T.RequestEntropy(%d)", oracle, signBlock)(oracle.RequestEntropy(sim.Acc(requester), signBlock))
			})

			// It's safe to access this from both the "provider" and the
			// "consumer" parallel tests because the consumer waits for an
			// event that is only triggered once the provider has finished.
			var providedEntropy [32]byte

			t.Run("provider", func(t *testing.T) {
				t.Parallel()

				select {
				case req, ok := <-entropyRequests:
					if !ok {
						t.Fatalf("%T closed unexpectedly", entropyRequests)
					}

					buf := httpGet(t, tt.signingServer, fmt.Sprintf("/%d", req.BlockNumber))
					sig := common.Hex2Bytes(string(buf))
					copy(providedEntropy[:], crypto.Keccak256Hash(sig).Bytes())

					e := []entropy.EntropyOracleEntropyFulfilment{{
						BlockNumber: provideBlock,
						Signature:   sig,
					}}
					if _, err := oracle.ProvideEntropy0(sim.Acc(public), e); (err != nil) != tt.wantErr {
						t.Errorf("%T.ProvideEntropy([as public], %+v) got error %v; want error: %t", oracle, e, err, tt.wantErr)
					}

				case err := <-entropyRequestErrs:
					t.Fatalf("%T error channel received %v", &entropy.EntropyOracleEntropyRequested{}, err)
				}
			})

			t.Run("consumer", func(t *testing.T) {
				if tt.wantErr {
					return
				}
				t.Parallel()

				select {
				case gotEv, ok := <-entropyProvisions:
					if !ok {
						t.Fatalf("%T closed unexpectedly", entropyProvisions)
					}

					t.Run("event", func(t *testing.T) {
						opts := []cmp.Option{
							cmpopts.IgnoreFields(*gotEv, "Raw"),
							cmp.AllowUnexported(*gotEv.BlockNumber),
						}
						wantEv := &entropy.EntropyOracleV2EntropyProvided{
							BlockNumber: provideBlock,
							Entropy:     providedEntropy,
						}
						if diff := cmp.Diff(wantEv, gotEv, opts...); diff != "" {
							t.Errorf("Emitted event diff (-want +got):\n%s", diff)
						}
					})

					t.Run("stored", func(t *testing.T) {
						got, err := oracle.BlockEntropy(nil, provideBlock)
						if want := providedEntropy; err != nil || got != want {
							t.Fatalf("%T.BlockEntropy(%d) got %#x, err = %v; want %#x, err = nil", oracle, provideBlock, got, err, want)
						}
					})

				case <-entropyProvisionErrs:
					t.Fatalf("%T error channel received %v", &entropy.EntropyOracleV2EntropyProvided{}, err)
				}
			})
		})
	}
}

type entropyEvent interface {
	*entropy.EntropyOracleEntropyRequested |
		*entropy.EntropyOracleEntropyProvided |
		*entropy.EntropyOracleV2EntropyRequested |
		*entropy.EntropyOracleV2EntropyProvided
}

// watch subscribes to events using a Watch<Event> function from an
// EntropyOracle. The returned error channel is from event.Subscription.Err().
func watch[E entropyEvent](t *testing.T, watch func(*bind.WatchOpts, chan<- E, []*big.Int) (event.Subscription, error)) (<-chan E, <-chan error) {
	t.Helper()

	ch := make(chan E)
	sub, err := watch(nil, ch, nil)
	if err != nil {
		var ev E
		t.Fatalf("watch for %T events: %v", ev, err)
	}
	t.Cleanup(func() {
		sub.Unsubscribe()
		close(ch)
	})

	return ch, sub.Err()
}

func TestOnlyMinedBlocks(t *testing.T) {
	block := new(uint64)
	server := newTestServer(
		t,
		func(context.Context) (uint64, error) { return *block, nil },
		entropySrc{},
		simBackendChainID,
	)

	const (
		ok       = http.StatusOK
		verboten = http.StatusForbidden
	)

	// NOTE: the tests are deliberately NOT hermetic to demonstrate that the
	// server only fetches the latest block if a request is greater than any
	// signed in the past.
	tests := []struct {
		setBlock, reqBlock uint64
		wantCode           int
		wantCached         bool
	}{
		{
			setBlock: 1,
			reqBlock: 5,
			wantCode: verboten,
		},
		{
			setBlock: 3,
			reqBlock: 3,
			wantCode: ok,
		},
		{
			reqBlock: 4,
			wantCode: verboten,
		},
		{
			setBlock: 100,
			reqBlock: 100,
			wantCode: ok,
		},
		{
			// The last test resulted in the latest block being cached as 100
			// so the server MUST NOT update this as reqBlock <= last setBlock.
			setBlock:   80,
			reqBlock:   100,
			wantCode:   ok,
			wantCached: true,
		},
	}

	for _, tt := range tests {
		t.Run("get signature", func(t *testing.T) {
			if tt.setBlock != 0 {
				*block = tt.setBlock
			}
			t.Logf("Block set to %d; HTTP GET /%d", *block, tt.reqBlock)
			url := fmt.Sprintf("%s/%d", server.URL, tt.reqBlock)

			res, err := server.Client().Get(url)
			if err != nil {
				t.Fatalf("%T.Client().Get(%q) error %v", server, url, err)
			}

			if got := res.StatusCode; got != tt.wantCode {
				t.Errorf("got status code %d; want %d", got, tt.wantCode)
			}

			gotCached, err := strconv.ParseBool(res.Header.Get(isCachedHeader))
			if err != nil || gotCached != tt.wantCached {
				t.Errorf("strconv.ParseBool(%T.Header.Get(%q)) got %t, err = %v; want %t, err = nil", res, isCachedHeader, gotCached, err, tt.wantCached)
			}
		})

		if t.Failed() {
			// Tests are deliberately not hermetic so a failure in one makes the
			// rest unreliable.
			break
		}
	}
}

// entropySrc implements prf.PRF using a crypto.KeccakState() BUT no key. It is
// NOT secure for use outside of tests. Its value is prepended to the state
// before any other write.
type entropySrc []byte

func (e entropySrc) ComputePRF(input []byte, outputLength uint32) ([]byte, error) {
	s := crypto.NewKeccakState()
	if _, err := s.Write(append(e, input...)); err != nil {
		return nil, fmt.Errorf("%T.Write(%v): %v", s, input, err)
	}

	buf := make([]byte, int(outputLength))
	if n, err := s.Read(buf); uint32(n) != outputLength || err != nil {
		return nil, fmt.Errorf("%T.Read([%d bytes]) got %d err=%v", s, outputLength, n, err)
	}
	return buf, nil
}
