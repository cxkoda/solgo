// Package gnosis provides helper functionality to work with gnosis multisig wallets and apps therein.
package gnosis

import (
	"github.com/ethereum/go-ethereum/common"
)

// TokenType denotes the different types of tokens that can be transferred using Gnosis' CSV app.
type TokenType string

// The different token types that can be transferred using Gnosis' CSV app.
const (
	ERC721  TokenType = "nft"
	ERC1155 TokenType = "nft"
	ERC20   TokenType = "erc20"
	ETH     TokenType = "native"
)

// GnosisCSVTransfer encodes a transfer with Gnosis' CSV app (https://github.com/bh2smith/safe-airdrop).
// Slices of this struct are intended to be converted into CSV format using
// `gocarina/gocsv` to be consumed by Gnosis' CSV app.
type GnosisCSVTransfer struct {
	TokenType    TokenType      `csv:"token_type"`
	TokenAddress common.Address `csv:"token_address"`
	Receiver     common.Address `csv:"receiver"`
	Amount       float64        `csv:"amount"`
	ID           uint64         `csv:"id"`
}

// NewERC1155Transfer creates a new ERC1155 transfer for Gnosis' CSV app.
func NewERC1155Transfer(collection common.Address, id uint64, amount uint64, receiver common.Address) *GnosisCSVTransfer {
	return &GnosisCSVTransfer{
		TokenType:    ERC721,
		TokenAddress: collection,
		Receiver:     receiver,
		Amount:       float64(amount),
		ID:           id,
	}
}

// NewERC721Transfer creates a new ERC721 transfer for Gnosis' CSV app.
func NewERC721Transfer(collection common.Address, id uint64, receiver common.Address) *GnosisCSVTransfer {
	return &GnosisCSVTransfer{
		TokenType:    ERC721,
		TokenAddress: collection,
		Receiver:     receiver,
		ID:           id,
	}
}

// NewERC20Transfer creates a new ERC20 transfer for Gnosis' CSV app.
func NewERC20Transfer(coin common.Address, amount float64, receiver common.Address) *GnosisCSVTransfer {
	return &GnosisCSVTransfer{
		TokenType:    ERC20,
		TokenAddress: coin,
		Receiver:     receiver,
		Amount:       amount,
	}
}

// NewETHTransfer creates a new ETH transfer for Gnosis' CSV app.
func NewETHTransfer(amount float64, receiver common.Address) *GnosisCSVTransfer {
	return &GnosisCSVTransfer{
		TokenType: ETH,
		Receiver:  receiver,
		Amount:    amount,
	}
}
