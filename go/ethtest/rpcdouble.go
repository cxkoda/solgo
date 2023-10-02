package ethtest

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/net/nettest"
)

// backendSubset is a subset of the methods defined by ethapi.Backend.
type backendSubset interface {
	BlockByNumber(context.Context, rpc.BlockNumber) (*types.Block, error)
	ChainConfig() *params.ChainConfig
	GetTd(context.Context, common.Hash) *big.Int
	HeaderByNumber(context.Context, rpc.BlockNumber) (*types.Header, error)
	StateAndHeaderByNumberOrHash(context.Context, rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error)
}

// An RPCStub implements stubbed eth_* RPC methods.
type RPCStub struct {
	b *stubBackend
}

// stubBackend implements the backendSubset interface via stubbing.
type stubBackend struct {
	chainID  uint64
	blockNum uint64

	// blocks defines the Block to be returned by BlockByNumber, keyed by
	// number. If a block <=BlockNum is requested but it is not in this map then
	// an empty Block is returned.
	blocks       map[uint64]*types.Block
	blocksByHash map[common.Hash]*types.Block
}

// NewRPCStub returns a new RPCStub.
func NewRPCStub(chainID, blockNum uint64) *RPCStub {
	return &RPCStub{
		b: &stubBackend{
			chainID:      chainID,
			blockNum:     blockNum,
			blocks:       make(map[uint64]*types.Block),
			blocksByHash: make(map[common.Hash]*types.Block),
		},
	}
}

// ServeHTTP starts an HTTP server that propagates all eth_* RPC requests to a
// new RPC server backed by s. It returns the HTTP server's address, with
// http:// prefix included.
func (s *RPCStub) ServeHTTP(tb testing.TB) string {
	return serveRPCHTTP(tb, s.b)
}

// serveRPCHTTP abstracts the functionality described by RPCStub.ServeHTTP()
func serveRPCHTTP(tb testing.TB, partial backendSubset) string {
	tb.Helper()

	// WARNING: hacky!
	//
	// We only have a subset of the full ethapi.Backend interface but need the
	// full one to pass to ethapi.NewBlockChainAPI(). This can be achived with
	// embedding whereby each method on a type is promoted to the type that
	// embeds it. We therefore embed backendSubset at the top level and
	// ethapi.Backend one level down so its (unimplemented) methods are masked
	// by the partial implementation. Note that if any of the unimplemented
	// methods is called there will be a panic.
	//
	// The alternative is to embed ethapi.Backend in every type that we accept
	// to this function. However this would break all type guarantees because if
	// a type attempts to override a method but has an incorrect signature, the
	// compiler won't know as it will simply see the embedded interface.
	type fallback struct{ ethapi.Backend }
	full := struct {
		backendSubset
		fallback
	}{backendSubset: partial}
	api := ethapi.NewBlockChainAPI(full)

	// The geth codebase will report minimal RPC errors in the return values but
	// logs the rest to log.Root(), so we need to intercept them.
	log.Root().SetHandler(log.FuncHandler(func(r *log.Record) error {
		if r.Lvl > log.LvlError {
			return nil
		}
		tb.Logf("%v", r)
		return nil
	}))

	lis, err := nettest.NewLocalListener("tcp")
	if err != nil {
		tb.Fatalf("nettest.NewLocalListener(tcp) error %v", err)
	}

	rpcSrv := rpc.NewServer()
	if err := rpcSrv.RegisterName("eth", api); err != nil {
		tb.Fatalf("%T.RegisterName(%q, ethapi.NewBlockChainAPI(%T)) error %v", rpcSrv, "eth", partial, err)
	}
	httpSrv := &http.Server{
		Handler: rpcSrv,
	}

	done := make(chan struct{})
	go func() {
		if got, want := httpSrv.Serve(lis), http.ErrServerClosed; got != want {
			tb.Errorf("%T.Serve(%T) got error %v; want %v", httpSrv, lis, got, want)
		}
		close(done)
	}()
	tb.Cleanup(func() {
		// lis is closed by httpSrv.Close(), per the last line of its
		// documentation.
		if err := httpSrv.Close(); err != nil {
			tb.Errorf("%T.Close() error %v", httpSrv, err)
		}
		rpcSrv.Stop()
		<-done
	})

	return fmt.Sprintf("http://%s", lis.Addr())
}

// SetBlockNumber sets the stub's current block number.
func (s *RPCStub) SetBlockNumber(n uint64) {
	s.b.blockNum = n
}

// AddBlocks adds all Blocks to the stub such that they will be returned by
// an RPC call to GetBlockBy{Number,Hash}(). If said methods are called with a
// Block <= s.BlockNum but it hasn't been added by this method, an empty Block
// is created and returned.
func (s *RPCStub) AddBlocks(tb testing.TB, bs ...*types.Block) {
	tb.Helper()

	for _, b := range bs {
		num := b.NumberU64()
		if _, ok := s.b.blocks[num]; ok {
			tb.Fatalf("%T.AddBlocks() with existing block %d; use ReplaceBlocks()", s, num)
		}
		s.b.blocks[num] = b
		s.b.blocksByHash[b.Hash()] = b
	}
}

// ReplaceBlocks replaces Blocks added by AddBlocks(). An explicit replacement
// is required to ensure deliberate behaviour in tests.
//
// Blocks are replaced by both number and hash.
func (s *RPCStub) ReplaceBlocks(tb testing.TB, bs ...*types.Block) {
	tb.Helper()

	for _, b := range bs {
		num := b.NumberU64()
		if _, ok := s.b.blocks[num]; !ok {
			tb.Fatalf("%T.ReplaceBlocks(number = %d) non-existent block; use AddBlocks()", s, num)
		}

		delete(s.b.blocksByHash, s.b.blocks[num].Hash())
		s.b.blocks[num] = b
		s.b.blocksByHash[b.Hash()] = b
	}
}

func (s *stubBackend) normaliseBlockNum(num rpc.BlockNumber) (rpc.BlockNumber, error) {
	curr := rpc.BlockNumber(s.blockNum)
	if num > curr {
		return 0, fmt.Errorf("cannot get future block %d; current block = %d", num, s.blockNum)
	}

	switch num {
	case rpc.LatestBlockNumber:
		return curr, nil
	case rpc.EarliestBlockNumber:
		return 0, nil
	case rpc.SafeBlockNumber, rpc.FinalizedBlockNumber, rpc.PendingBlockNumber:
		txt, _ := num.MarshalText() // all paths return nil error so it's safe to drop
		return 0, fmt.Errorf("special %T = %q not supported", num, txt)
	default:
		return num, nil
	}
}

func (s *stubBackend) BlockByNumber(ctx context.Context, num rpc.BlockNumber) (*types.Block, error) {
	num, err := s.normaliseBlockNum(num)
	if err != nil {
		return nil, err
	}

	b, ok := s.blocks[uint64(num)]
	if ok {
		return b, nil
	}

	hdr := &types.Header{
		Number: big.NewInt(num.Int64()),
		Extra:  []byte{}, // this is required because of weirdness in the geth code base
	}
	return types.NewBlock(hdr, nil /*txs*/, nil /*uncles*/, nil /*receipts*/, nil /*types.TrieHasher*/), nil
}

func (s *stubBackend) ChainConfig() *params.ChainConfig {
	return &params.ChainConfig{
		ChainID: new(big.Int).SetUint64(s.chainID),
	}
}

func (s *stubBackend) GetTd(ctx context.Context, hash common.Hash) *big.Int {
	b, ok := s.blocksByHash[hash]
	if !ok {
		return big.NewInt(0)
	}
	return b.Difficulty()
}

func (s *stubBackend) HeaderByNumber(ctx context.Context, num rpc.BlockNumber) (*types.Header, error) {
	num, err := s.normaliseBlockNum(num)
	if err != nil {
		return nil, err
	}
	return &types.Header{
		Number: big.NewInt(num.Int64()),
	}, nil
}

// ServeHTTP functions identically to RPCStub.ServeHTTP().
func (sb *SimulatedBackend) ServeHTTP(tb testing.TB) string {
	return serveRPCHTTP(tb, &simBackend{
		sim: sb.SimulatedBackend,
	})
}

// simBackend implements the backendSubset interface using a SimulatedBackend.
type simBackend struct {
	sim *backends.SimulatedBackend
}

func (s *simBackend) BlockByNumber(ctx context.Context, num rpc.BlockNumber) (*types.Block, error) {
	if num == rpc.LatestBlockNumber {
		bc := s.sim.Blockchain()
		return bc.GetBlockByHash(bc.CurrentBlock().Hash()), nil
	}
	return s.sim.BlockByNumber(ctx, big.NewInt(num.Int64()))
}

func (s *simBackend) ChainConfig() *params.ChainConfig {
	return s.sim.Blockchain().Config()
}

func (s *simBackend) GetTd(ctx context.Context, hash common.Hash) *big.Int {
	b, err := s.sim.BlockByHash(ctx, hash)
	if err != nil {
		log.Root().Error("%T.BlockByHash(%v): %v", ctx, s.sim, hash, err)
		return nil
	}
	return b.Difficulty()
}

func (s *simBackend) HeaderByNumber(ctx context.Context, num rpc.BlockNumber) (*types.Header, error) {
	if num == rpc.LatestBlockNumber {
		return s.sim.Blockchain().CurrentHeader(), nil
	}
	return s.sim.HeaderByNumber(ctx, big.NewInt(num.Int64()))
}

// UnimplementedStubMethod is an error returned by a stubbed method that exists
// to avoid panics if called but is otherwise unimplemented.
type UnimplementedStubMethod struct {
	Method string
	Stub   reflect.Type
}

// Error implements the error interface.
func (u *UnimplementedStubMethod) Error() string {
	return fmt.Sprintf("RPC stub method %q not implemented by %s", u.Method, u.Stub.String())
}

func (s *stubBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	return nil, nil, &UnimplementedStubMethod{
		Method: "StateAndHeaderByNumberOrHash",
		// Note that we use the exported type, not the internal *stubBackend, to
		// allow for checking in tests.
		Stub: reflect.TypeOf(&RPCStub{}),
	}
}

func (s *simBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	return nil, nil, &UnimplementedStubMethod{
		Method: "StateAndHeaderByNumberOrHash",
		Stub:   reflect.TypeOf(s.sim), // see comment on *stubBackend equivalent
	}
}
