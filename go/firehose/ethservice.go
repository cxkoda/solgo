package firehose

import (
	"context"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	filterpb "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/transform/v1"
	sfethpb "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	hosepb "github.com/streamingfast/pbgo/sf/firehose/v2"

	"github.com/cxkoda/solgo/go/secrets"
	svcpb "github.com/cxkoda/solgo/projects/indexing/firehose/proto/eth"
	ethpb "github.com/cxkoda/solgo/proto/eth"
)

type ethHandler struct {
	proxy *Proxy[*sfethpb.Block]
}

type ethServer struct {
	*ethHandler
}

type ethClient struct {
	*ethHandler
}

func newETHHandler(ctx context.Context, endpointURL, tokenURL, apiKey string, opts ...grpc.DialOption) (*ethHandler, func() error, error) {
	proxy, err := Dial[*sfethpb.Block](ctx, endpointURL, tokenURL, apiKey, opts...)
	if err != nil {
		return nil, func() error { return nil }, fmt.Errorf("Dial(): %v", err)
	}
	return &ethHandler{proxy: proxy}, proxy.Close, nil
}

// ETHServer returns a new Ethereum Hydrant service server.
func ETHServer(ctx context.Context, endpointURL, tokenURL, apiKey string, opts ...grpc.DialOption) (svcpb.HydrantServiceServer, func() error, error) {
	h, cleanup, err := newETHHandler(ctx, endpointURL, tokenURL, apiKey, opts...)
	if err != nil {
		return nil, cleanup, err
	}
	return &ethServer{h}, cleanup, err
}

// ETHMainnetServer returns a new Ethereum Hydrant service server connected to
// the Mainnet endpoint.
func ETHMainnetServer(ctx context.Context, apiKey string, opts ...grpc.DialOption) (svcpb.HydrantServiceServer, func() error, error) {
	return ETHServer(ctx, ETHMainnetURL, TokenURL, apiKey, opts...)
}

// ETHGoerliServer returns a new Ethereum Hydrant service server connected to
// the Goerli endpoint.
func ETHGoerliServer(ctx context.Context, apiKey string, opts ...grpc.DialOption) (svcpb.HydrantServiceServer, func() error, error) {
	return ETHServer(ctx, ETHGoerliURL, TokenURL, apiKey, opts...)
}

// ETHClient returns a new Ethereum Hydrant service client.
func ETHClient(ctx context.Context, endpointURL, tokenURL, apiKey string, opts ...grpc.DialOption) (svcpb.HydrantServiceClient, func() error, error) {
	h, cleanup, err := newETHHandler(ctx, endpointURL, tokenURL, apiKey, opts...)
	if err != nil {
		return nil, cleanup, err
	}
	return &ethClient{h}, cleanup, err
}

// ETHClientFromSecret returns a new Ethereum Hydrant service client for the given chain ID
// using the provided secret.
func ETHClientFromSecret(ctx context.Context, chainID uint64, apiKey *secrets.Secret) (svcpb.HydrantServiceClient, func() error, error) {
	// TODO(@aschlosberg): create configuration struct.
	var dial func(context.Context, string, ...grpc.DialOption) (svcpb.HydrantServiceClient, func() error, error)

	switch chainID {
	case 1:
		dial = ETHMainnetClient
	case 5:
		dial = ETHGoerliClient
	default:
		return nil, nil, fmt.Errorf("unsupported chain ID: %d", chainID)
	}

	key, err := apiKey.Fetch(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("apiKey.Fetch(ctx): %v", err)
	}
	client, cleanup, err := dial(ctx, string(key))
	if err != nil {
		return nil, nil, fmt.Errorf("dial(ctx, %v) %v", string(key), err)
	}
	return client, cleanup, err
}

// ETHMainnetClient returns a new Ethereum Hydrant service client connected to
// the Mainnet endpoint.
func ETHMainnetClient(ctx context.Context, apiKey string, opts ...grpc.DialOption) (svcpb.HydrantServiceClient, func() error, error) {
	return ETHClient(ctx, ETHMainnetURL, TokenURL, apiKey, opts...)
}

// ETHGoerliClient returns a new Ethereum Hydrant service client connected to
// the Goerli endpoint.
func ETHGoerliClient(ctx context.Context, apiKey string, opts ...grpc.DialOption) (svcpb.HydrantServiceClient, func() error, error) {
	return ETHClient(ctx, ETHGoerliURL, TokenURL, apiKey, opts...)
}

// Events implements the HydrantService.Events method.
func (s *ethServer) Events(req *svcpb.EventsRequest, resp svcpb.HydrantService_EventsServer) error {
	return s.events(resp.Context(), req, resp.Send)
}

// ERC721TransferEvents implements the HydrantService.ERC721TransferEvents
// method. It overrides req.Signature with the appropriate ERC721 signature but
// otherwise functions identically to the generic Events() method.
func (s *ethServer) ERC721TransferEvents(req *svcpb.EventsRequest, resp svcpb.HydrantService_ERC721TransferEventsServer) error {
	req, err := withERC721TransferSig(req)
	if err != nil {
		return err
	}
	return s.events(resp.Context(), req, resp.Send)
}

// withERC721TransferSig sets req's Signatures to ERC721TransferEvent() and
// returns req. It returns an error if there are already Signatures.
func withERC721TransferSig(req *svcpb.EventsRequest) (*svcpb.EventsRequest, error) {
	if len(req.Signatures) != 0 {
		return nil, status.Error(codes.InvalidArgument, "%T must not have Signatures for pre-defined event")
	}
	req.Signatures = []*ethpb.Event{ERC721TransferEvent()}
	return req, nil
}

// ERC721TransferEvent returns the protobuf signature of an ERC721 Transfer
// event.
func ERC721TransferEvent() *ethpb.Event {
	return &ethpb.Event{
		Name: "Transfer",
		Arguments: []*ethpb.Argument{
			ethpb.NewArgument("from", &ethpb.Value_Address{}, true),
			ethpb.NewArgument("to", &ethpb.Value_Address{}, true),
			ethpb.NewArgument("tokenId", &ethpb.Value_Uint256{}, true),
		},
	}
}

// Events implements the HydrantService.Events method. The returned client is
// not a complete grpc.ClientStream and only implements the Recv() method. All
// other methods will panic.
//
// To avoid leaking a goroutine and a channel, Recv() MUST be called until it
// returns a non-nil error, be that due to context cancellation, end of stream
// indicated by io.EOF, or a true error.
func (c *ethClient) Events(ctx context.Context, req *svcpb.EventsRequest, opts ...grpc.CallOption) (svcpb.HydrantService_EventsClient, error) {
	return c.newETHAdaptor(ctx, req), nil
}

// ERC721TransferEvents implements the HydrantService.ERC721TransferEvents
// method. It overrides req.Signature with the appropriate ERC721 signature but
// otherwise functions identically to the generic Events() method.
//
// See Events() re not leaking a goroutine.
func (c *ethClient) ERC721TransferEvents(ctx context.Context, req *svcpb.EventsRequest, opts ...grpc.CallOption) (svcpb.HydrantService_ERC721TransferEventsClient, error) {
	req, err := withERC721TransferSig(req)
	if err != nil {
		return nil, err
	}
	return c.newETHAdaptor(ctx, req), nil
}

// An ethAdaptor converts a HydrantService_EventsServer into a
// HydrantService_EventsClient, in-process. It only supports the the Recv()
// method, which returns the BlockResponse received on `blocks` or returns `err`
// if said channel is closed.
type ethAdaptor struct {
	blocks <-chan *svcpb.BlockResponse
	err    error
	// Embedded to allow this to be a drop-in replacement for streaming clients.
	grpc.ClientStream
}

func (c *ethClient) newETHAdaptor(ctx context.Context, req *svcpb.EventsRequest) *ethAdaptor {
	// A single goroutine is spawned by this function. It is responsible for
	// sending on (and hence closing) the BlockResponse channel although sending
	// has a level of indirection via the send() function passed to c.events().
	ch := make(chan *svcpb.BlockResponse)
	a := &ethAdaptor{
		blocks: ch,
	}

	send := func(b *svcpb.BlockResponse) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- b:
			return nil
		}
	}
	go func() {
		defer close(ch)
		switch err := c.events(ctx, req, send); err {
		case nil:
			a.err = io.EOF
		default:
			a.err = err
		}
	}()

	return a
}

// Recv receives a block on the a.blocks channel and returns it. The channel is
// closed by the goroutine in newETHAdaptor when ethClient.events() returns and
// populates a.err; after which, any current and future calls to Recv() will
// return said error. Receiving a non-nil error here is therefore proof that the
// goroutine has finished, hence the comment on Events() re avoiding leaks.
func (a *ethAdaptor) Recv() (*svcpb.BlockResponse, error) {
	b, ok := <-a.blocks
	if !ok {
		return nil, a.err
	}
	return b, nil
}

// A blockResponseStreamer is a gRPC server stream of BlockResponses.
type blockResponseStreamer interface {
	Send(*svcpb.BlockResponse) error
	grpc.ServerStream
}

// events is the common logic shared by all <T>Events methods.
//
// Logging verbosity:
// - always log new valid request
// - V(1) start/end of block, block stream (Firehose connection), and tx
// - V(2) data parsing and conversion
func (s *ethHandler) events(ctx context.Context, req *svcpb.EventsRequest, send func(*svcpb.BlockResponse) error) error {
	if err := req.Validate(); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	sigs := make([][]byte, len(req.Signatures))
	sigStrings := make([]string, len(req.Signatures))

	// Although these aren't used until later, they act as extra validation.
	extractors := make(ethEventExtractors)
	for i, sig := range req.Signatures {
		x, err := newEthEventExtractor(sig)
		if err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		extractors[x.hash] = x
		sigs[i] = x.hash.Bytes()
		sigStrings[i] = sig.EVMString()
	}

	contracts := make(addressSet)
	filter := &filterpb.LogFilter{
		EventSignatures: sigs,
		Addresses:       make([][]byte, len(req.Contracts)),
	}
	for i, addr := range req.Contracts {
		contracts[common.BytesToAddress(addr.Bytes)] = struct{}{}
		filter.Addresses[i] = addr.Bytes
	}
	glog.Infof("Fetching %q events emitted by %#x", sigStrings, filter.Addresses)

	transform, err := anypb.New(&filterpb.CombinedFilter{
		LogFilters: []*filterpb.LogFilter{filter},
	})
	if err != nil {
		return fmt.Errorf("anypb.New(%T): %v", &filterpb.CombinedFilter{}, err)
	}

	blockReq := &hosepb.Request{
		StartBlockNum: req.StartBlockNum,
		StopBlockNum:  req.StopBlockNum,
		Transforms:    []*anypb.Any{transform},
		Cursor:        req.Cursor,
	}
	blocks, err := s.proxy.Blocks(ctx, blockReq)
	if err != nil {
		return fmt.Errorf("%T.Blocks(): %v", s.proxy, err)
	}
	defer blocks.Close()
	glog.V(1).Info("Block stream opened")

	var sentBlocks, sentTxs int
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case b, ok := <-blocks.C:
			if !ok {
				glog.V(1).Infof("Block stream closed; sent %d transaction(s) across %d block(s)", sentTxs, sentBlocks)
				return blocks.Err()
			}

			block, err := extractors.extract(b.Block, contracts)
			if err != nil {
				return err
			}
			out := &svcpb.BlockResponse{
				Block:         block,
				Cursor:        b.Response.Cursor,
				FirehoseBlock: b.Block,
				FirehoseStep:  b.Response.Step,
			}
			if err := send(out); err != nil {
				return err
			}

			glog.V(1).Infof("Sent block %d", out.Block.Number)
			sentBlocks++
			sentTxs += len(out.Block.Transactions)
		}
	}
}

type addressSet map[common.Address]struct{}

// contains returns whether the ETH address represented by b is contained in the
// set. It uses the memory array underlying b instead of copying it; len(b) MUST
// therefore == common.AddressLength.
func (s addressSet) contains(b []byte) bool {
	if len(b) != common.AddressLength {
		glog.Fatalf("%T.contains() called with slice of length %d", s, len(b))
	}
	_, ok := s[*(*common.Address)(b)]
	return ok
}

type ethEventExtractors map[common.Hash]*ethEventExtractor

// extract parses the StreamingFast ETH block and converts it into a Hydrant ETH
// block. The primary functionality is to find the correct event extractor for
// each log and use it to extract structured data from the raw bytes.
func (exs ethEventExtractors) extract(b *sfethpb.Block, contracts addressSet) (*ethpb.Block, error) {
	glog.V(1).Infof("Parsing block %d", b.Number)

	block := &ethpb.Block{
		Number:    b.Number,
		TimeStamp: b.Header.Timestamp,
		Hash: &ethpb.Hash{
			Bytes: b.Hash,
		},
	}

	for _, tx := range b.TransactionTraces {
		glog.V(1).Infof("Parsing tx %#x", tx.Hash)

		var events []*ethpb.Event
		for _, log := range tx.Receipt.Logs {
			// If Firehose matches an event signature + contract address, it
			// returns all logs of that signature within the transaction. This
			// causes ERC20 and ERC721 Transfers to be returned together as they
			// have the same signature, even if one comes from a different
			// contract (e.g. purchasing an ERC721 with wETH).
			if !contracts.contains(log.Address) || len(log.Topics) == 0 {
				continue
			}

			xtractor, ok := exs[common.BytesToHash(log.Topics[0])]
			if !ok {
				continue
			}
			ev, err := xtractor.asEvent(log)
			if err != nil {
				return nil, fmt.Errorf("tx %#x: log index %d: %v", tx.Hash, log.Index, err)
			}
			events = append(events, ev)
		}

		if len(events) == 0 {
			continue
		}

		block.Transactions = append(block.Transactions, &ethpb.Transaction{
			Hash: &ethpb.Hash{Bytes: tx.Hash},
			Logs: events,
		})
	}

	return block, nil
}

// An ethEventExtractor finds and parses topics and raw log data matching a
// specific signature.
type ethEventExtractor struct {
	sig  *ethpb.Event
	hash common.Hash

	args           abi.Arguments
	indexed        abi.Arguments
	nonIndexed     abi.Arguments
	argIndexByName map[string]int
}

func newEthEventExtractor(sig *ethpb.Event) (*ethEventExtractor, error) {
	args, err := sig.ABIArguments()
	if err != nil {
		return nil, err
	}

	var indexed abi.Arguments
	byName := make(map[string]int)
	for i, a := range args {
		if a.Indexed {
			indexed = append(indexed, a)
		}

		n := a.Name
		if n == "" {
			n = fmt.Sprintf(":unnamed_arg:[%d]", i)
		}
		if _, ok := byName[n]; ok {
			return nil, fmt.Errorf("duplicate argument %q", n)
		}
		byName[n] = i
	}

	return &ethEventExtractor{
		sig:            proto.Clone(sig).(*ethpb.Event),
		hash:           sig.EVMHash(),
		args:           args,
		indexed:        indexed,
		nonIndexed:     args.NonIndexed(),
		argIndexByName: byName,
	}, nil
}

func (e *ethEventExtractor) asEvent(log *sfethpb.Log) (*ethpb.Event, error) {
	// We explicitly don't support anonymous events so Topics[0] is always the
	// event signature.
	if n, m := len(log.Topics), len(e.indexed); n != m+1 {
		return nil, fmt.Errorf("%d topics for %d indexed arguments; expecting %d", n, m, m+1)
	}

	event := proto.Clone(e.sig).(*ethpb.Event)
	event.Emitter = &ethpb.Address{Bytes: log.Address}
	event.LogIndex = log.Index
	glog.V(2).Infof("Emitting contract: %#x", log.Address)

	// ======
	// TOPICS
	// ======

	topics := make([]common.Hash, 0, len(e.indexed))
	for _, t := range log.Topics[1:] {
		topics = append(topics, common.BytesToHash(t))
	}
	parsed := make(map[string]interface{})
	if err := abi.ParseTopicsIntoMap(parsed, e.indexed, topics); err != nil {
		return nil, err
	}
	glog.V(2).Infof("Topics: %+v", parsed)

	for name, val := range parsed {
		if err := e.setArgument(event, name, val); err != nil {
			return nil, status.Errorf(status.Code(err), fmt.Sprintf("setting topic: %v", err))
		}
	}

	// ====
	// DATA
	// ====

	data, err := e.args.Unpack(log.Data)
	if err != nil {
		return nil, fmt.Errorf("%T.Unpack(%T.Data = %#x): %v", e.args, log, log.Data, err)
	}
	glog.V(2).Infof("Data: %v", data)

	for i, val := range data {
		if err := e.setArgument(event, e.nonIndexed[i].Name, val); err != nil {
			return nil, status.Errorf(status.Code(err), fmt.Sprintf("setting data argument [%d]: %v", i, err))
		}
	}

	// ===============
	// Name => arg map
	// ===============
	args := make(map[string]*ethpb.Argument)
	for _, a := range event.Arguments {
		args[a.Name] = a
	}
	event.ArgumentsByName = args

	return event, nil
}

func (e *ethEventExtractor) setArgument(ev *ethpb.Event, name string, val interface{}) error {
	idx, ok := e.argIndexByName[name]
	if !ok {
		// This would mean a serious bug so use codes.Internal
		return status.Errorf(codes.Internal, "unknown argument name %q", name)
	}
	glog.V(2).Infof("arg[%d] (%q) => %T(%v)", idx, name, val, val)
	return ev.Arguments[idx].Value.SetPayload(val)
}
