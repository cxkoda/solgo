// Package usbwallet abstracts functionality provided by go-ethereum's usbwallet
// package, providing fully managed access to hardware wallets.
package usbwallet

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/usbwallet"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
	"github.com/hashicorp/go-multierror"

	proofsync "github.com/cxkoda/solgo/go/sync"
)

// Type defines a type of USB wallet.
type Type int

// Available wallet Types.
const (
	UnknownWalletType Type = iota
	Ledger
)

// A Wallet manages hardware wallets connected over USB. As it manages device
// events concurrently, any errors are reported on a channel accessible via the
// Err() method.
type Wallet struct {
	hub      hub
	basePath accounts.DerivationPath
	typ      Type
	errs     <-chan error

	quit          chan struct{}
	eventLoopDone chan struct{}

	// Using a single-item buffer in lieu of a sync.Mutex as this gives greater
	// flexibility when working with context cancellation. Lock() <=> read from
	// channel; Unlock() <=> send back to channel. "Share by communicating" is a
	// Go mantra; also see:
	// https://drive.google.com/file/d/1nPdvhB0PutEJzdCq5ms6UI58dp50fcAN/view
	wallets chan map[accounts.URL]*walletAndStatus

	// Wallet.eventLoop() ensures that the Toggle's state is equivalent to
	// anyOf([w.open() for w in wallets]). Wallet.Wait() is therefore merely a
	// wrapper around available.Wait().
	available *proofsync.Toggle
}

// hub defines the minimal set of usbwallet.Hub methods needed by this package,
// used for injecting test doubles.
type hub interface {
	Subscribe(chan<- accounts.WalletEvent) event.Subscription
	Wallets() []accounts.Wallet
}

type walletAndStatus struct {
	accounts.Wallet
	url accounts.URL
	// Although the physical hardware device may be connected and "open", the
	// Ethereum-specific app may not be. This is definitely the case on Ledgers;
	// if extending to support hardware for which this isn't the case, simply
	// couple the two values.
	deviceOpen, appOpen bool
}

// open returns true iff the device is connected and the Ethereum app is opened.
func (w walletAndStatus) open() bool {
	return w.deviceOpen && w.appOpen
}

// errChanBuffer is the number of errors to store before dropping them.
const errChanBuffer = 16

// New creates a new Wallet backed by the Hub, connected to the specific Type of
// hardware.
func New(hub *usbwallet.Hub, t Type, basePath accounts.DerivationPath) *Wallet {
	return construct(hub, t, basePath)
}

// construct abstracts New() to allow for testing with a test-double
// implementation of usbwallet.Hub. Without this we'd need to expose the
// arbitrary hub interface, obscuring any documentation coupling it to
// usbwallet.Hub.
func construct(hub hub, t Type, basePath accounts.DerivationPath) *Wallet {
	errCh := make(chan error, errChanBuffer)
	w := &Wallet{
		hub:           hub,
		basePath:      basePath,
		typ:           t,
		errs:          errCh,
		wallets:       make(chan map[accounts.URL]*walletAndStatus, 1),
		quit:          make(chan struct{}),
		eventLoopDone: make(chan struct{}),
		available:     new(proofsync.Toggle),
	}
	w.wallets <- make(map[accounts.URL]*walletAndStatus)
	go w.listen(errCh)

	return w
}

// Wait blocks until at least one hardware device is connected and ready to sign
// signable things. In the case of a Ledger device, this means that not only is
// the device unlocked, but its Ethereum app is open.
func (w *Wallet) Wait(ctx context.Context) error {
	return w.available.Wait(ctx)
}

// Err returns the channel on which all errors are reported. Errors are not
// fatal to the Wallet.
func (w *Wallet) Err() <-chan error {
	return w.errs
}

// Close stops all device management and frees resources, also closing the Err()
// channel and unblocking all Wait() calls with sync.ErrToggleClosed.
func (w *Wallet) Close() error {
	close(w.quit)
	w.available.Close()
	wallets := <-w.wallets
	close(w.wallets)

	<-w.eventLoopDone

	var e *multierror.Error
	for url, ww := range wallets {
		if ww.deviceOpen {
			glog.Infof("[%v] closing", url)
			e = multierror.Append(e, ww.Close())
		}
	}
	return e.ErrorOrNil()
}
