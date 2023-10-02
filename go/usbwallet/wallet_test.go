package usbwallet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestWallet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	preConnected := &fakeDevice{label: "pre-connected"}
	hub := newFakeHub(t, preConnected)

	w := construct(hub, Ledger, accounts.DefaultBaseDerivationPath)
	if err := w.Wait(ctx); err != nil {
		t.Fatalf("%T.Wait() with pre-connected device: %v", w, err)
	}

	done := make(chan struct{})
	defer func() {
		cancel()
		<-done
	}()
	t.Run("Err() channel", func(t *testing.T) {
		go func() {
			defer close(done)
			for {
				select {
				case <-ctx.Done():
					return
				case err, ok := <-w.Err():
					if !ok {
						return
					}
					t.Errorf("%T.Err() received %v", w, err)
				}
			}
		}()
	})

	t.Run("SignerFn", func(t *testing.T) {
		t.Run("nil expected address", func(t *testing.T) {
			testSignerFn(t, w, preConnected, 0, nil)
		})
		t.Run("zero expected address", func(t *testing.T) {
			var zero common.Address
			testSignerFn(t, w, preConnected, 0, &zero)
		})
	})

	var extraDevices []*fakeDevice
	t.Run("concurrent device arrival", func(t *testing.T) {
		// Synchronise all go routines so we hammer the concurrency handling
		// and try to induce a race condition. Gotta hand it to Rust here ;)
		start := make(chan struct{})
		var wg sync.WaitGroup
		defer func() {
			close(start)
			wg.Wait()
		}()

		const n = 100
		for i := 0; i < n; i++ {
			dev := &fakeDevice{
				hub:   hub,
				label: fmt.Sprintf("device-%d", i),
			}

			// Typically a device "arrives" and the event loop calls Open(), but
			// it's valid for an already-open device to join.
			evType := accounts.WalletArrived
			if i%3 == 0 {
				if err := dev.Open(""); err != nil {
					t.Fatalf("%T.Open() to connect already-open device; error %v", dev, err)
				}
				evType = accounts.WalletOpened
			}

			hub.devices = append(hub.devices, dev)
			extraDevices = append(extraDevices, dev)

			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				hub.subCh <- accounts.WalletEvent{
					Wallet: dev,
					Kind:   evType,
				}

				if err := dev.open.Wait(ctx); err != nil {
					t.Errorf("%T.open.Wait(ctx) error %v", dev, err)
				}
			}()
		}
	})

	t.Run("all devices opened by event loop", func(t *testing.T) {
		for _, w := range hub.devices {
			dev := w.(*fakeDevice)
			if !dev.open.State() {
				t.Errorf("%T{%q}.Open() not called", w, dev.label)
			}
		}
	})

	if t.Failed() {
		// The tests for closed devices will have spurious results because not
		// all are open.
		return
	}

	t.Run("dropped devices are closed", func(t *testing.T) {
		t.Run("previously unseen are ignored", func(t *testing.T) {
			hub.subCh <- accounts.WalletEvent{
				Wallet: &urlOnlyDevice{}, // will panic if Close() is called
				Kind:   accounts.WalletDropped,
			}
		})

		start := make(chan struct{})
		var wg sync.WaitGroup
		defer func() {
			close(start)
			wg.Wait()
		}()

		for i, w := range hub.devices {
			if i%13 == 0 {
				continue
			}
			wg.Add(1)
			go func(dev *fakeDevice) {
				defer wg.Done()
				<-start

				hub.subCh <- accounts.WalletEvent{
					Wallet: dev,
					Kind:   accounts.WalletDropped,
				}
				<-dev.activelyClosed
			}(w.(*fakeDevice))
		}
	})

	t.Run("unambiguous derivation for SignerFn", func(t *testing.T) {
		if _, _, err := w.SignerFn(0, nil, big.NewInt(0)); err != ErrAmbiguousDerivation {
			t.Errorf("got %v; want %v", err, ErrAmbiguousDerivation)
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		defer func() {
			close(start)
			wg.Wait()
		}()

		for i, dev := range extraDevices {
			if !dev.open.State() {
				continue
			}
			index := uint32(i)
			// As we have multiple devices connected, an expected address is
			// required so all devices can be checked.
			expected := dev.deriveAddrT(t, w.derivationPath(index))

			wg.Add(1)
			go func(dev *fakeDevice) {
				defer wg.Done()
				<-start
				testSignerFn(t, w, dev, index, &expected)
			}(dev)
		}
	})

	if err := w.Close(); err != nil {
		t.Fatalf("%T.Close() error %v", w, err)
	}

	t.Run("all devices closed by Close()", func(t *testing.T) {
		for _, w := range hub.devices {
			w := w.(*fakeDevice)
			if w.open.State() {
				t.Errorf("%T{%q}.Close() not called", w, w.label)
			}
		}
	})
}

// testSignerFn tests Wallet.SignerFn(). The fakeDevice MUST be the instance
// expected to derive the account.
func testSignerFn(t *testing.T, w *Wallet, dev *fakeDevice, index uint32, expectedAddr *common.Address) {
	t.Helper()

	chainID := big.NewInt(1337)

	wantAddr := dev.deriveAddrT(t, w.derivationPath(index))
	fn, gotAddr, err := w.SignerFn(index, expectedAddr, chainID)
	if err != nil || gotAddr != wantAddr {
		t.Fatalf("%T.SignerFn(%d, nil, %d) got %v, err = %v; want %v, nil err", w, index, chainID, gotAddr, err, wantAddr)
	}

	data := []byte("must-be-propagated")
	tx := types.NewTransaction(
		42,               // nonce
		common.Address{}, // to
		big.NewInt(0),    // amount
		21_000,           // guess
		big.NewInt(0),    // gas price
		data,
	)

	signed, err := fn(gotAddr, tx)
	if err != nil {
		t.Fatalf("%T.SignerFn(0)(tx) error %v", w, err)
	}
	if got, want := signed.Data(), data; !bytes.Equal(got, want) {
		t.Fatalf("%T.SignerFn() does not propagate transaction; got data %q; want originally sent value %q", w, got, want)
	}

	t.Run("signed by", func(t *testing.T) {
		s := signer(chainID)
		got, err := types.Sender(s, signed)
		if err != nil {
			t.Fatalf("types.Sender(%T, [tx signed by w.SignerFn()]) error %v", s, err)
		}

		if got != wantAddr {
			t.Errorf("%T.SignerFn()() returned tx; got types.Sender() = %v; want %v", w, got, wantAddr)
		}
	})
}

func TestWalletErrorPropagation(t *testing.T) {
	hub := newFakeHub(t)

	w := construct(hub, Ledger, accounts.DefaultBaseDerivationPath)
	defer w.Close()

	err := errors.New("uh oh")
	hub.subErrs <- err

	got := <-w.Err()
	if !errors.Is(got, err) {
		t.Errorf("%T.Err() received %v; want %v (exact error sent on subscription channel)", w, got, err)
	}

	t.Run("full buffer drops new errors without blocking", func(t *testing.T) {
		for i := 0; i < errChanBuffer+10; i++ {
			hub.subErrs <- err
		}
	})
}
