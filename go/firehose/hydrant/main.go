// The hydrant binary connects to the StreamingFast Firehose service to expose
// simplified APIs.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"time"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/cxkoda/solgo/go/proof"
	"github.com/cxkoda/solgo/go/secrets"

	"github.com/cxkoda/solgo/projects/indexing/firehose"
	svcpb "github.com/cxkoda/solgo/projects/indexing/firehose/proto/eth"
)

const (
	Mainnet = "mainnet"
	Goerli  = "goerli"
)

func main() {
	cfg := config{
		firehoseAPIKey: proof.FirehoseAPIKey(),
	}

	flag.IntVar(&cfg.port, "port", 8080, "Port on which to listen for gRPC reqRuests")
	flag.Var(cfg.firehoseAPIKey, "firehose_api_key", "Firehose API Key")
	flag.StringVar(&cfg.ethChain, "eth_chain", Mainnet, "Ethereum Chain (mainnet or goerli)")
	flag.DurationVar(&cfg.grpcStreamTimeout, "grpc_stream_timeout", 0, "gRPC stream timeout")
	flag.Parse()

	if err := cfg.run(context.Background()); err != nil {
		glog.Exit(err)
	}
}

type config struct {
	ethChain          string
	firehoseAPIKey    *secrets.Secret
	grpcStreamTimeout time.Duration
	port              int
}

func (cfg *config) run(ctx context.Context) (retErr error) {
	var opts []grpc.DialOption
	var dial func(ctx context.Context, apiKey string, opts ...grpc.DialOption) (svcpb.HydrantServiceServer, func() error, error)

	switch cfg.ethChain {
	case Mainnet:
		dial = firehose.ETHMainnetServer
	case Goerli:
		dial = firehose.ETHGoerliServer
	default:
		return fmt.Errorf("unknown network %q", cfg.ethChain)
	}

	if cfg.grpcStreamTimeout > 0 {
		opts = append(opts, grpc.WithTimeout(cfg.grpcStreamTimeout))
	}

	firehoseAPIKey, err := cfg.firehoseAPIKey.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("%T(%q).Fetch(): %v", cfg.firehoseAPIKey, cfg.firehoseAPIKey.String(), err)
	}

	srv, cleanup, err := dial(ctx, string(firehoseAPIKey), opts...)
	if err != nil {
		return err
	}
	defer func() {
		if err := cleanup(); retErr == nil {
			retErr = err
		}
	}()

	s := grpc.NewServer()
	svcpb.RegisterHydrantServiceServer(s, srv)
	reflection.Register(s)

	addr := fmt.Sprintf(":%d", cfg.port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("net.Listen(tcp, %q): %v", addr, err)
	}
	fmt.Println(fmt.Sprintf("hydrant service listening on [%s] network with port [%d]", cfg.ethChain, cfg.port))

	return s.Serve(lis)
}
