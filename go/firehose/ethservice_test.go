package firehose_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/google/go-cmp/cmp"
	"github.com/h-fam/errdiff"
	"github.com/holiman/uint256"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/proofxyz/solgo/projects/indexing/firehose"
	"github.com/proofxyz/solgo/projects/indexing/firehose/firehosetest"

	svcpb "github.com/proofxyz/solgo/projects/indexing/firehose/proto/eth"
	ethpb "github.com/proofxyz/solgo/proto/eth"
)

func TestEvents(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		config firehosetest.Config
	}{
		{
			name: "ETHServer",
			config: firehosetest.Config{
				UseETHServer: true,
			},
		},
		{
			name: "ETHClient",
			config: firehosetest.Config{
				UseETHServer: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := tt.config.NewFake(ctx, t)

			emitterAddr, _, emit, err := DeployEmitter(fake.TxOpts(), fake.Backend())
			if err != nil {
				t.Fatalf("DeployEmitter(…) error %v", err)
			}

			// To demonstrate filtering by contract address; i.e. ignoring anything
			// emitted by this instance.
			_, _, emit2, err := DeployEmitter(fake.TxOpts(), fake.Backend())
			if err != nil {
				t.Fatalf("DeployEmitter(…) error %v", err)
			}

			from := common.HexToAddress("0xc0ffee")
			to := common.HexToAddress("0xdead")
			const tokenID = 42
			var transferTx *types.Transaction

			for id, contract := range map[int64]*Emitter{
				tokenID:     emit,
				tokenID + 1: emit2,
			} {
				tx, err := contract.Transfer(fake.TxOpts(), from, to, big.NewInt(id))
				if err != nil {
					t.Fatalf("%T.Transfer(%v, %v, %v) error %v", contract, from, to, tokenID, err)
				}
				if id == tokenID {
					transferTx = tx
				}
			}

			data := []byte("hello world")
			dataTx, err := emit.WithData(fake.TxOpts(), tokenID, data)
			if err != nil {
				t.Fatalf("%T.WithData(%d, %q) error %v", emit, tokenID, string(data), err)
			}

			mined := fake.MineBlock(ctx, t)

			req := &svcpb.EventsRequest{
				Contracts: []*ethpb.Address{{Bytes: emitterAddr.Bytes()}},
				Signatures: []*ethpb.Event{
					firehose.ERC721TransferEvent(),
					{
						Name: "WithData",
						Arguments: []*ethpb.Argument{
							ethpb.NewArgument("topic", &ethpb.Value_Uint8{}, true),
							ethpb.NewArgument("data", &ethpb.Value_Bytes{}, false),
						},
					},
				},
			}
			blocks, err := fake.Client.Events(ctx, req)
			if err != nil {
				t.Fatalf("%T.Client.Events(%+v) error %v", fake, req, err)
			}

			got := firehosetest.CollectAll(t, blocks)

			want := []*svcpb.BlockResponse{{
				Block: &ethpb.Block{
					Number:    mined.NumberU64(),
					TimeStamp: &timestamppb.Timestamp{Seconds: int64(mined.Time())},
					Hash:      &ethpb.Hash{Bytes: mined.Hash().Bytes()},
					Transactions: []*ethpb.Transaction{
						{
							Hash: &ethpb.Hash{Bytes: transferTx.Hash().Bytes()},
							Logs: []*ethpb.Event{
								ethpb.NewEvent(
									"Transfer", emitterAddr,
									firehosetest.Arg(t, "from", from, true),
									firehosetest.Arg(t, "to", to, true),
									firehosetest.Arg(t, "tokenId", uint256.MustFromBig(big.NewInt(tokenID)), true),
								),
							},
						},
						{
							Hash: &ethpb.Hash{Bytes: dataTx.Hash().Bytes()},
							Logs: []*ethpb.Event{
								ethpb.NewEvent(
									"WithData", emitterAddr,
									firehosetest.Arg(t, "topic", uint8(tokenID), true),
									ethpb.NewArgument("data", &ethpb.Value_Bytes{Bytes: data}, false),
								),
							},
						},
					},
				},
				Cursor: firehosetest.Cursor(mined),
			}}

			ignore := protocmp.IgnoreFields(&ethpb.Event{}, "log_index")

			if diff := cmp.Diff(want, got, firehosetest.CmpOpts(), ignore); diff != "" {
				t.Errorf("All blocks received by %T.Events(%+v) diff (-want +got):\n%s", fake.Client, req, diff)
			}
		})
	}
}

func TestETHClientRecvError(t *testing.T) {
	// Calling ETHClient doesn't return a real gRPC client stream, but one that
	// simply adapts the server returned by ETHServer. It's therefore more
	// important to ensure the unhappy path is tested.

	ctx := context.Background()

	cfg := firehosetest.Config{UseETHServer: false}
	fake := cfg.NewFake(ctx, t)

	// We happen to check for invalid requests inside ethHandler.events()
	// because that's the common point shared by all other methods. This is the
	// error propagation we're testing, and also the easiest one to invoke.
	invalidReq := &svcpb.EventsRequest{
		Contracts: []*ethpb.Address{{
			Bytes: make([]byte, common.AddressLength+1),
		}},
	}
	blocks, err := fake.Client.ERC721TransferEvents(ctx, invalidReq)
	if err != nil {
		t.Fatalf("%T.Client.ERC721TransferEvents(%+v) error %v", fake, invalidReq, err)
	}

	_, err = blocks.Recv()
	if diff := errdiff.Code(err, codes.InvalidArgument); diff != "" {
		t.Errorf("%T.Client.ERC721TransferEvents([invalid contract address]).Recv() %s", fake, diff)
	}
}
