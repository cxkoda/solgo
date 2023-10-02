package usbwallet

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/glog"
)

// errQuit is returned by eventLoop() to signal to listen() that it returned
// early because the Wallet.quit channel was closed.
var errQuit = errors.New("quit event loop")

// listen Subscribe()s to the underlying usbwallet.Hub and takes ownership of
// errCh. It returns when w.quit is closed, at which point it closes errCh.
func (w *Wallet) listen(errCh chan<- error) {
	ch := make(chan accounts.WalletEvent)
	sub := w.hub.Subscribe(ch)

	done := make(chan struct{})
	go w.trackExistingWallets(ch, done)

	defer func() {
		sub.Unsubscribe()
		close(ch)
		close(errCh)
		close(w.eventLoopDone)
		// We're only here if w.quit has closed, which also causes
		// trackExistingWallets() to return early.
		<-done
	}()

	for {
		switch err := w.eventLoop(sub, ch); err {
		case nil:
		case errQuit:
			return
		default:
			select {
			case errCh <- err:
			default: // Drop if the buffer is full
			}
		}
	}
}

// trackExistingWallets sends WalletArrived events for all of w.hub.Wallets()
// then closes done. This causes eventLoop() to become aware of USB devices
// opened before the Wallet was constructed.
func (w *Wallet) trackExistingWallets(ch chan<- accounts.WalletEvent, done chan struct{}) {
	defer close(done)

	for _, ww := range w.hub.Wallets() {
		ev := accounts.WalletEvent{
			Wallet: ww,
			Kind:   accounts.WalletArrived,
		}

		select {
		case <-w.quit:
			return
		case ch <- ev:
		}
	}
}

// ledgerAppOnlineRE matches the status returned by a Ledger Nano when in the
// Ethereum app, not the "home" screen.
var ledgerAppOnlineRE = regexp.MustCompile(`^Ethereum app v\d+\.\d+\.\d+ online$`)

// eventLoop handles a single iteration of the event loop. The channel MUST be
// the same one used to create the Subscription. An loop iteration may be any
// one of:
//
// 1. w.quit is closed, returning errQuit to signal to listen() to return.
// 2. An error is received on sub.Err(), which is returned.
// 3. An event is received on ch, at which point wallet bookkeeping is handled.
func (w *Wallet) eventLoop(sub event.Subscription, ch <-chan accounts.WalletEvent) error {
	select {
	case <-w.quit:
		return errQuit

	case err, ok := <-sub.Err():
		if !ok {
			return fmt.Errorf("%T.Err() close unexpectedly", sub)
		}
		return fmt.Errorf("%T.Err(): %w", sub, err)

	case ev, ok := <-ch:
		if !ok {
			return fmt.Errorf("%T closed unexpectedly", ch)
		}

		// The URL is not unique to the device itself and changes even within
		// the span of a "session" (for example, opening the Ethereum app on a
		// Ledger will cause a drop and then new arrival with different URL).
		url := ev.Wallet.URL()
		log := func(msg string) {
			glog.InfoDepth(1, fmt.Sprintf("[%v] %s", url, msg))
		}

		var wallets map[accounts.URL]*walletAndStatus
		select {
		case wallets = <-w.wallets:
		case <-w.quit:
			return errQuit
		}
		defer func() {
			w.wallets <- wallets

			// Set the w.available Toggle to (un)block current and future
			// callers to Wallet.Wait().
			var available bool
			for _, ww := range wallets {
				if ww.open() {
					available = true
					break
				}
			}
			w.available.Set(available) // idempotent
		}()

		switch ev.Kind {
		case accounts.WalletArrived:
			log("arrived")
			wallets[url] = &walletAndStatus{Wallet: ev.Wallet, url: url}

			if err := ev.Wallet.Open("" /*passphrase*/); err != nil {
				return fmt.Errorf("%T{%v}.Open(%q): %w", ev.Wallet, url, "", err)
			}

		case accounts.WalletOpened:
			log("opened")
			ww, ok := wallets[url]
			if !ok {
				log("previously unseen")
				ww = &walletAndStatus{Wallet: ev.Wallet, url: url}
				wallets[url] = ww
			}

			ww.deviceOpen = true

			status, err := ev.Wallet.Status()
			if err != nil {
				return fmt.Errorf("%T.Status(): %v", ev.Wallet, err)
			}
			switch {
			case status == "Ethereum app offline":
				log("app offline")
			case ledgerAppOnlineRE.MatchString(status):
				log("app online")
				ww.appOpen = true
			default:
				return fmt.Errorf("unrecognised status %q of %q", status, url)
			}

		case accounts.WalletDropped:
			log("dropped")
			ww, ok := wallets[url]
			if !ok {
				log("previously unseen; not closing")
				return nil
			}

			if err := ww.Close(); err != nil {
				return fmt.Errorf("%T{%v}.Close(): %w", ww, url, err)
			}
			delete(wallets, url)
			log("closed and deleted")

		default:
			return fmt.Errorf("unrecognised %T = %d", ev.Kind, ev.Kind)
		}
	}

	return nil
}
