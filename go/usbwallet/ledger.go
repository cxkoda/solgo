package usbwallet

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/usbwallet"
)

// NewLedger is equivalent to calling New() with parameters specific to Ledger
// devices.
func NewLedger() (*Wallet, error) {
	hub, err := usbwallet.NewLedgerHub()
	if err != nil {
		return nil, fmt.Errorf("go-ethereum/accounts/usbwallet.NewLedgerHub(): %w", err)
	}
	return New(hub, Ledger, accounts.DefaultBaseDerivationPath), nil
}
