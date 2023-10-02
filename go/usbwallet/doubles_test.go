package usbwallet

import (
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"math/big"
	"path"
	"testing"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"

	"github.com/proofxyz/solgo/go/sync"
)

// fakeHub implements the hub interface, satisfying the minimum required methods
// of a *usbwallet.Hub.
type fakeHub struct {
	devices []accounts.Wallet

	subCh   chan<- accounts.WalletEvent
	subErrs chan error
}

func newFakeHub(t *testing.T, preloaded ...*fakeDevice) *fakeHub {
	h := &fakeHub{
		subErrs: make(chan error),
	}
	t.Cleanup(func() {
		close(h.subErrs)
	})

	for _, d := range preloaded {
		d.hub = h
		h.devices = append(h.devices, d)
	}
	return h
}

func (h *fakeHub) Subscribe(ch chan<- accounts.WalletEvent) event.Subscription {
	h.subCh = ch
	return &fakeSub{
		errs: h.subErrs,
	}
}

func (h *fakeHub) Wallets() []accounts.Wallet {
	return h.devices
}

// fakeSub implements event.Subscription.
type fakeSub struct {
	errs <-chan error
}

func (s *fakeSub) Err() <-chan error {
	return s.errs
}

func (s *fakeSub) Unsubscribe() {
}

// A urlOnlyDevice only has the URL method implemented. All other calls will
// panic for nil-pointer derefence.
type urlOnlyDevice struct {
	accounts.Wallet
	label string
}

func (d *urlOnlyDevice) URL() accounts.URL {
	return accounts.URL{
		Scheme: "test-double",
		Path:   d.label,
	}
}

// fakeDevice implements accounts.Wallet. It is called a device and not a wallet
// to avoid confusion with the Wallet type provided by this package.
type fakeDevice struct {
	accounts.Wallet

	hub   *fakeHub
	label string

	open       sync.Toggle
	passphrase string

	// Unlike `open`, which encodes a state, this channel signals the closure
	// of the device via Close(). It is constructed by Open().
	activelyClosed chan struct{}

	pins map[common.Address]*ecdsa.PrivateKey
}

func (d *fakeDevice) URL() accounts.URL {
	return accounts.URL{
		Scheme: "test-double",
		Path:   d.label,
	}
}

func (d *fakeDevice) Open(passphrase string) error {
	if d.open.State() {
		return fmt.Errorf("%T{%q}.Open() when already open", d, d.label)
	}

	d.passphrase = passphrase
	d.activelyClosed = make(chan struct{})

	go func() {
		d.hub.subCh <- accounts.WalletEvent{
			Wallet: d,
			Kind:   accounts.WalletOpened,
		}
		// This MUST be toggled here, after sending the event, as a means of
		// synchronising with the test code, which calls d.open.Wait().
		d.open.Set(true)
	}()
	return nil
}

func (d *fakeDevice) Status() (string, error) {
	return "Ethereum app v0.0.0 online", nil
}

func (d *fakeDevice) Close() error {
	if !d.open.State() {
		return fmt.Errorf("%T{%q}.Close() when not open", d, d.label)
	}
	d.open.Set(false)
	close(d.activelyClosed)
	return nil
}

func (d *fakeDevice) Derive(dPath accounts.DerivationPath, pin bool) (_ accounts.Account, retErr error) {
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("deriving key for device label=%q; passphrase=%q; derivation path=%v: %v", d.label, d.passphrase, dPath, retErr)
		}
	}()

	// TODO(arran): use a real HD wallet implementation if one is available.
	// miguelmota/go-ethereum-hdwallet is in maintenance mode so doesn't compile
	// with other dependencies. For now it suffices to just use a deterministic
	// pseudo-random function derived (by arbitrary method) from "device"
	// parameters.

	state := crypto.NewKeccakState()
	for _, d := range []any{
		uint64(len(d.label)),
		[]byte(d.label),
		uint64(len(d.passphrase)),
		[]byte(d.passphrase),
		dPath,
	} {
		if err := binary.Write(state, binary.BigEndian, d); err != nil {
			return accounts.Account{}, fmt.Errorf("binary.Write(%T, , %T): %v", state, d, err)
		}
	}

	curve := crypto.S256()
	key, err := ecdsa.GenerateKey(curve, state)
	if err != nil {
		return accounts.Account{}, fmt.Errorf("ecdsa.GenerateKey(%T, %T): %v", curve, state, err)
	}
	addr := crypto.PubkeyToAddress(key.PublicKey)

	if pin {
		if d.pins == nil {
			d.pins = make(map[common.Address]*ecdsa.PrivateKey)
		}
		d.pins[addr] = key
	}

	return accounts.Account{
		Address: addr,
		URL: accounts.URL{
			Scheme: d.URL().Scheme,
			Path:   path.Join(d.URL().Path, dPath.String()),
		},
	}, nil
}

func (d *fakeDevice) SignTx(acc accounts.Account, tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
	if d.pins == nil || d.pins[acc.Address] == nil {
		return nil, fmt.Errorf("%T requested to sign for unpinned address %v", d, acc.Address)
	}
	return types.SignTx(tx, signer(chainID), d.pins[acc.Address])
}

func signer(chainID *big.Int) types.Signer {
	return types.NewLondonSigner(chainID)
}

// Below here are helper functions not required for the interfaces, but useful
// for testing.

func (d *fakeDevice) deriveT(tb testing.TB, path accounts.DerivationPath) accounts.Account {
	tb.Helper()
	acc, err := d.Derive(path, false)
	if err != nil {
		tb.Fatalf("%T{%v}.Derive(%v, false): %v", d, d.label, path, err)
	}
	return acc
}

func (d *fakeDevice) deriveAddrT(tb testing.TB, path accounts.DerivationPath) common.Address {
	tb.Helper()
	acc := d.deriveT(tb, path)
	return acc.Address
}
