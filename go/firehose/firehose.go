// Package firehose implements simplified APIs for accessing the StreamingFast
// Firehose gRPC service. See https://firehose.streamingfast.io/
package firehose

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"

	"github.com/proofxyz/solgo/go/oauthsrc"

	sfethpb "github.com/streamingfast/firehose-ethereum/types/pb/sf/ethereum/type/v2"
	hosepb "github.com/streamingfast/pbgo/sf/firehose/v2"
	solpb "github.com/streamingfast/sf-solana/types/pb/sf/solana/type/v2"
)

// Production URLs for the hosted Firehose service.
const (
	TokenURL      = "https://auth.dfuse.io/v1/auth/issue"
	ETHMainnetURL = "mainnet.eth.streamingfast.io:443"
	ETHGoerliURL  = "goerli.eth.streamingfast.io:443"
)

// BlockProto is a type constraint limited to the types of blocks supported by
// Firehose.
type BlockProto interface {
	*sfethpb.Block | *solpb.Block
}

// A Proxy provides access to the Firehose service through a modified API.
type Proxy[B BlockProto] struct {
	conn   *grpc.ClientConn
	client hosepb.StreamClient
}

// Close closes the underlying connection to the Firehose service.
func (p *Proxy[_]) Close() error {
	return p.conn.Close()
}

// DialETHMainnet is equivalent to Dial() with URLs to connect to the Ethereum
// Mainnet endpoint and sfethpb.Block as the type argument.
func DialETHMainnet(ctx context.Context, apiKey string, opts ...grpc.DialOption) (*Proxy[*sfethpb.Block], error) {
	return Dial[*sfethpb.Block](ctx, ETHMainnetURL, TokenURL, apiKey, opts...)
}

// DialETHGoerli is equivalent to Dial() with URLs to connect to the Ethereum
// Goerli endpoint and sfethpb.Block as the type argument.
func DialETHGoerli(ctx context.Context, apiKey string, opts ...grpc.DialOption) (*Proxy[*sfethpb.Block], error) {
	return Dial[*sfethpb.Block](ctx, ETHGoerliURL, TokenURL, apiKey, opts...)
}

// Dial dials the gRPC endpoint with tokens obtained from the token URL. If no
// gRPC DialOptions are provided then DefaultDialOptions(…) is used.
func Dial[B BlockProto](ctx context.Context, endpointURL, tokenURL, apiKey string, opts ...grpc.DialOption) (*Proxy[B], error) {
	if len(opts) == 0 {
		o, err := DefaultDialOptions(tokenURL, apiKey)
		if err != nil {
			return nil, err
		}
		opts = o
	}

	conn, err := grpc.DialContext(ctx, endpointURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("grpc.DialContext(%q): %v", endpointURL, err)
	}
	return &Proxy[B]{
		conn:   conn,
		client: hosepb.NewStreamClient(conn),
	}, nil
}

// DefaultDialOptions returns gRPC DialOptions suitable for calling production
// Firehose servers as run by StreamingFast.
func DefaultDialOptions(tokenURL, apiKey string) ([]grpc.DialOption, error) {
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("x509.SystemCertPool(): %v", err)
	}

	return []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			RootCAs: roots,
		})),
		grpc.WithPerRPCCredentials(oauth.TokenSource{
			TokenSource: oauth2.ReuseTokenSource(
				nil,
				oauthsrc.NewHTTP(
					"POST", tokenURL, fmt.Sprintf(`{"api_key":%q}`, apiKey), &dfuseToken{},
				),
			),
		}),
	}, nil
}

// dfuseToken represents the JSON token returned by the dfuse auth server.
type dfuseToken struct {
	Payload   string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// AsToken returns the dfuseToken as an oauth2.Token.
func (t *dfuseToken) AsToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken: t.Payload,
		TokenType:   "Bearer",
		Expiry:      time.Unix(t.ExpiresAt, 0),
	}
}

// Blocks queries propagates the request to the Proxy's underlying client.
func (p *Proxy[B]) Blocks(ctx context.Context, req *hosepb.Request, opts ...grpc.CallOption) (*Blocks[B], error) {
	stream, err := p.client.Blocks(ctx, req, opts...)
	if err != nil {
		return nil, fmt.Errorf("%T.Blocks(%+v): %v", p.client, req, err)
	}

	blocks := make(chan Block[B])
	ret := &Blocks[B]{
		C:    blocks,
		quit: make(chan struct{}),
	}
	go func() (retErr error) {
		defer func() {
			ret.err = retErr
			close(blocks)
		}()

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("%T.Recv(): %v", stream, err)
			}

			pb, err := resp.Block.UnmarshalNew()
			if err != nil {
				return fmt.Errorf("%T.Block.UnmarshalNew(): %v", resp, err)
			}
			block, ok := pb.(B)
			if !ok {
				return fmt.Errorf("%T.Block.UnmarshalNew() got %T; want %T", resp, pb, block)
			}

			select {
			case blocks <- Block[B]{Response: resp, Block: block}:
			case <-ret.quit:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}()

	return ret, nil
}

// A Blocks stream provides a channel of blocks.
type Blocks[B BlockProto] struct {
	// C is a channel on which new Blocks are sent. It is closed when either the
	// stream client ends or an error occurs.
	C    <-chan Block[B]
	quit chan struct{}
	err  error
}

// An Block couples a Firehose response with an unmarshalled block of concrete
// type.
type Block[B BlockProto] struct {
	Response *hosepb.Response
	Block    B
}

// Err returns any error that occurred during streaming. It SHOULD be called
// after C is closed.
func (b *Blocks[_]) Err() error {
	return b.err
}

// Close MUST be called to release resources used by the block streamer.
func (b *Blocks[_]) Close() {
	close(b.quit)
}
