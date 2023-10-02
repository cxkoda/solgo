package eth

import (
	"context"
	"flag"
	"fmt"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/proofxyz/solgo/go/secrets"
)

// A Dialer dials an Ethereum node URL that it sources from a secret store. This
// is to protect API keys that may be stored in the URL.
type Dialer struct {
	nodeURL    *secrets.Secret
	secretOpts []secrets.Option
}

// DialerFlag is the flag name configured by NewDialerFromFlags.
const DialerFlag = "dialer_eth_node_url"

func MustNewDialerFromFlag(fs *flag.FlagSet, defaultNodeURL *secrets.Secret, opts ...secrets.Option) *Dialer {
	d, err := NewDialerFromFlag(fs, defaultNodeURL, opts...)
	if err != nil {
		panic(err)
	}
	return d
}

// NewDialerFromFlag returns a Dialer that is configurable via command-line
// flags; see DialerFlag. The Options are propagated when Fetch()ing the Secret.
func NewDialerFromFlag(fs *flag.FlagSet, defaultNodeURL *secrets.Secret, opts ...secrets.Option) (*Dialer, error) {
	if fs.Parsed() {
		return nil, fmt.Errorf("%T already parsed", fs)
	}

	var url secrets.Secret
	if defaultNodeURL != nil {
		url = *defaultNodeURL
	}
	fs.Var(&url, DialerFlag, "URL of Ethereum node to dial, stored as a secrets.Secret; e.g. env://NODE_URL or gcp://path/to/secret")

	return NewDialer(&url, opts...), nil
}

// NewDialer returns a Dialer that sources its node URL from the given Secret.
func NewDialer(nodeURL *secrets.Secret, opts ...secrets.Option) *Dialer {
	return &Dialer{nodeURL: nodeURL, secretOpts: opts}
}

// Dial Fetch()es the Dialer's secret node URL and returns
// ethclient.DialContext(ctx, [secret]).
func (c *Dialer) Dial(ctx context.Context) (*ethclient.Client, error) {
	url, err := c.nodeURL.Fetch(ctx, c.secretOpts...)
	if err != nil {
		return nil, fmt.Errorf("%T(%q).Fetch(…): %v", c.nodeURL, c.nodeURL.String(), err)
	}
	return ethclient.DialContext(ctx, string(url))
}

// An RWDemuxBackend splits calls to ContractBackend methods to a read-only and
// write-only backend. This is useful when using a regular node (e.g. Infura)
// for reading, but Flashbots for writing.
//
// Everything except for SendTransaction() and EstimateGas() are sent to the
// read backend. EstimateGas is included as a "write" because it may leak
// sensitive information.
func RWDemuxBackend(r, w bind.ContractBackend) bind.ContractBackend {
	return rwDemuxBackend{
		ContractBackend: r,
		w:               w,
	}
}

type rwDemuxBackend struct {
	bind.ContractBackend
	w bind.ContractBackend
}

// SendTransaction is equivalent to the same method on the write-only
// ContractBackend.
func (b rwDemuxBackend) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	return b.w.SendTransaction(ctx, tx)
}

// EstimateGas is equivalent to the same method on the write-only
// ContractBackend.
func (b rwDemuxBackend) EstimateGas(ctx context.Context, call ethereum.CallMsg) (gas uint64, err error) {
	return b.w.EstimateGas(ctx, call)
}
