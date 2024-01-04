// The entropyserver binary…
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
	"golang.org/x/time/rate"

	"github.com/cxkoda/solgo/contracts/go/hotsigner"
	"github.com/cxkoda/solgo/go/eth"
	"github.com/cxkoda/solgo/go/secrets"

	_ "embed"
)

func main() {
	var cfg config
	flag.IntVar(&cfg.port, "port", 8080, "Port on which to listen for HTTP requests.")
	flag.Var(&cfg.ethRPCURL, "eth_rpc_url", "Ethereum RPC URL source; e.g. env://INFURA_MAINNET_WITH_KEY.")
	flag.DurationVar(&cfg.blockInterval, "block_interval", 12*time.Second, "Interval at which blocks are mined, to rate limit calls to fetch latest block number.")
	flag.Parse()

	if err := cfg.run(context.Background()); err != nil {
		glog.Exit(err)
	}
}

type config struct {
	port          int
	ethRPCURL     secrets.Secret
	blockInterval time.Duration
}

func (cfg *config) run(ctx context.Context) error {
	//
	// ETH Client
	//
	nodeURL, err := cfg.ethRPCURL.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("%T(%q).Fetch(): %v", cfg.ethRPCURL, cfg.ethRPCURL.String(), err)
	}
	ethClient, err := ethclient.DialContext(ctx, string(nodeURL))
	if err != nil {
		return fmt.Errorf("ethclient.DialContext(…, [redacted URL from %q]): %v", cfg.ethRPCURL.String(), err)
	}
	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		return fmt.Errorf("%T.ChainID(): %v", ethClient, err)
	}

	chain := uint256Bytes(chainID.Uint64())
	signer, err := hotsigner.New(ctx, chain, 0)
	if err != nil {
		return fmt.Errorf("hotsigner.New(ctx, %#x, 0): %v", chain, err)
	}

	//
	// Glue
	//
	src, err := newSource(
		ethClient.BlockNumber,
		cfg.blockInterval,
		signer,
		chainID.Uint64(),
	)
	if err != nil {
		return fmt.Errorf("newSource(): %v", err)
	}

	addr := fmt.Sprintf(":%d", cfg.port)
	glog.Infof("Listening on %q for chain %d", addr, src.chainID)
	return http.ListenAndServe(addr, src)
}

// A blockSource returns the latest block number mined on a blockchain.
type blockSource func(context.Context) (uint64, error)

// A source signs block numbers i.f.f. they have already been mined.
type source struct {
	signer  *eth.Signer
	chainID uint64

	latestBlock blockSource
	currBlock   *atomic.Uint64
	limiter     *rate.Limiter
}

// newSource constructs a new source using the pseudorandom function to
// determine a signer private key.
//
// NOTE that using a prf.PRF offers sufficient security only for these purposes
// (i.e. short-lived, no assets owned by the address). If a more secure signer
// is needed, GCP KMS supports secp256k1.
func newSource(blockSrc blockSource, blockInterval time.Duration, s *eth.Signer, chainID uint64) (*source, error) {
	return &source{
		signer:      s,
		chainID:     chainID,
		latestBlock: blockSrc,
		currBlock:   new(atomic.Uint64),
		limiter:     rate.NewLimiter(rate.Every(blockInterval), 4),
	}, nil
}

// uint256Bytes returns the 32-byte array representing x as if it were
// interpreted as a uint256 in Solidity.
func uint256Bytes(x uint64) []byte {
	return math.U256Bytes(new(big.Int).SetUint64(x))
}

var (
	errOnlyGetMethod   = errors.New("only GET requests")
	errNonNumericBlock = errors.New("non-numeric block number")
	errNegativeBlock   = errors.New("negative block number")
	errBlockNotMined   = errors.New("block not yet mined")
)

const signerAddrEndpoint = "/signer"

// ServeHTTP implements the http.Handler interface. All requests are handled by
// s.sign().
func (s *source) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Add("X-Chain-Id", fmt.Sprintf("%d", s.chainID))
	h.Add("Access-Control-Allow-Origin", "*")

	fn := s.sign
	if r.Method == http.MethodGet && r.URL.Path == signerAddrEndpoint {
		fn = s.signerAddr
	}

	switch code, err := fn(w, r); code {
	case 200:
	case http.StatusMethodNotAllowed, http.StatusBadRequest, http.StatusForbidden:
		http.Error(w, err.Error(), code)
	default:
		key := sha256.Sum256([]byte(err.Error()))
		glog.Errorf("[%x] %v", key[:8], err)
		http.Error(w, fmt.Sprintf("Uh oh; please report %x", key[:8]), code)
	}
}

// signerAddr writes, to w, the Ethereum address of the block signer.
func (s *source) signerAddr(w http.ResponseWriter, _ *http.Request) (int, error) {
	if _, err := hex.NewEncoder(w).Write(s.signer.Address().Bytes()); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

// sign reads the block number from the request path, and signs it i.f.f. the
// block has already been mined.
func (s *source) sign(w http.ResponseWriter, r *http.Request) (int, error) {
	// NOTE that http.StatusForbidden is reservered for blocks not yet mined so
	// we can use the code for testing.

	if r.Method != http.MethodGet {
		return http.StatusMethodNotAllowed, errOnlyGetMethod
	}

	reqBlock, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
	if err != nil {
		return http.StatusBadRequest, errNonNumericBlock
	}
	if reqBlock < 0 {
		return http.StatusBadRequest, errNegativeBlock
	}

	mined, err := s.blockMined(r.Context(), w, uint64(reqBlock))
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if !mined {
		return http.StatusForbidden, errBlockNotMined
	}

	buf := uint256Bytes(uint64(reqBlock))
	buf = append(buf, uint256Bytes(s.chainID)...)
	sig, err := s.signer.PersonalSign(buf)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("%T.RawSign(%#x): %v", s.signer, buf, err)
	}

	if _, err := hex.NewEncoder(w).Write(sig); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("hex.NewEncoder(%T).Write([signature]): %v", w, err)
	}
	return http.StatusOK, nil
}

const isCachedHeader = "X-Cached-Block-Number"

// blockMined returns whether the block is known to have already be mined. It
// caches the latest-mined block and will update this iff the source's rate
// limiter allows.
func (s *source) blockMined(ctx context.Context, w http.ResponseWriter, block uint64) (bool, error) {
	w.Header().Add(isCachedHeader, "true")

	curr := s.currBlock.Load()
	if curr >= block {
		return true, nil
	}
	// To avoid DoS vectors, we avoid extra calls to the node if we know for
	// sure that there's been insufficient time since the last check.
	if !s.limiter.Allow() {
		return false, nil
	}

	w.Header().Set(isCachedHeader, "false")
	latest, err := s.latestBlock(ctx)
	if err != nil {
		return false, fmt.Errorf("fetch latest block number: %v", err)
	}
	s.currBlock.CompareAndSwap(curr, latest)

	return latest >= block, nil
}
