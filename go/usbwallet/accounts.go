package usbwallet

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/golang/glog"
)

var zeroAddr common.Address

// SignerFn returns a SignerFn for use in a bind.TransactOpts. Unlike Metamask,
// the account index is 0-based. As the Wallet can manage more that one hardware
// device, the expected address removes ambiguity but MAY be nil (or a pointer
// to the zero address) i.f.f. there is only a single USB device connected.
//
// Although a SignerFn accepts a types.Transaction, which itself contains a
// chain ID, the returned function is bound to a pre-specified chain for added
// security.
func (w *Wallet) SignerFn(index uint32, expectedAddr *common.Address, chainID *big.Int) (bind.SignerFn, common.Address, error) {
	ww, acc, err := w.derive(index, expectedAddr)
	if err != nil {
		return nil, zeroAddr, err
	}

	// Clone to avoid the pointer being changed.
	chainID = new(big.Int).Set(chainID)

	return func(signAddr common.Address, tx *types.Transaction) (*types.Transaction, error) {
		// Don't allow the eventloop to modify the wallets.
		x := <-w.wallets
		defer func() {
			w.wallets <- x
		}()

		if signAddr != acc.Address {
			return nil, fmt.Errorf("signing for %v with account %v", signAddr, acc.Address)
		}
		if !ww.open() {
			return nil, fmt.Errorf("%T closed since account %v pinned", ww.Wallet, acc.Address)
		}

		glog.Infof(
			"[%v][%v] signing tx=%#x to=%v nonce=%d value=%d data=%#x",
			ww.url, acc.Address, tx.Hash(), tx.To(), tx.Nonce(), tx.Value(), tx.Data(),
		)
		tx, err := ww.SignTx(acc, tx, chainID)
		if err != nil {
			return nil, fmt.Errorf("%T.SignTx(%+v, %+v, %d): %v", ww.Wallet, acc, tx, chainID, err)
		}
		glog.Infof("[%v] signed tx %#x as %v", ww.url, tx.Hash(), acc.Address)
		return tx, nil
	}, acc.Address, nil
}

var (
	// ErrAmbiguousDerivation is returned when account derivation is attempted
	// while multiple devices are connected but no non-zero expected address is
	// provided.
	ErrAmbiguousDerivation = errors.New("ambiguous account derivation")
	// ErrNoWalletsOpen is returned when account derivation is attempted but no
	// devices are open.
	ErrNoWalletsOpen = errors.New("no wallets open")
)

// derive checks all open() wallets for an account at the specified index that
// matches the expected address. See SignerFn() re behaviour of nil/zero
// expectedAddr to avoid ambiguity.
func (w *Wallet) derive(index uint32, expectedAddr *common.Address) (*walletAndStatus, accounts.Account, error) {
	var addr common.Address
	if expectedAddr != nil {
		addr = *expectedAddr
	}
	path := w.derivationPath(index)

	wallets := <-w.wallets
	defer func() {
		w.wallets <- wallets
	}()

	switch n := len(wallets); n {
	case 0:
		return nil, accounts.Account{}, ErrNoWalletsOpen
	case 1:
	default:
		if addr == zeroAddr {
			return nil, accounts.Account{}, ErrAmbiguousDerivation
		}
	}

	for url, ww := range wallets {
		if !ww.open() {
			continue
		}

		acc, err := ww.Wallet.Derive(path, true)
		if err != nil {
			return nil, accounts.Account{}, fmt.Errorf("%T.Derive(%v, true): %v", ww.Wallet, path, err)
		}

		glog.Infof("[%v] derived %v", url, acc)
		if addr != zeroAddr && acc.Address != addr {
			continue
		}
		return ww, acc, nil
	}

	return nil, accounts.Account{}, fmt.Errorf("no account %d with address %v found", index, addr)
}

// derivationPath returns the DerivationPath for the 0-indexed account,
// accounting (pun intended) for the Type passed to New() when creating the
// Wallet.
func (w *Wallet) derivationPath(index uint32) accounts.DerivationPath {
	path := make(accounts.DerivationPath, len(w.basePath))
	copy(path, w.basePath)

	switch w.typ {
	case Ledger:
		path[2] += index
	default:
		path[len(path)-1] += index
	}

	return path
}
