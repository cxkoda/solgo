// Package gnosis provides helper functionality to work with gnosis multisig wallets and apps therein.
package gnosis

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
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

// EmptyAddress is a wrapper around `common.Address` that marshals as empty string if the address is the ZeroAddress
type EmptyAddress common.Address

func (a EmptyAddress) String() string {
	addr := common.Address(a)
	if addr == common.HexToAddress("") {
		return ""
	}
	return addr.String()
}

// GnosisCSVTransfer encodes a transfer with Gnosis' CSV app (https://github.com/bh2smith/safe-airdrop).
// Slices of this struct are intended to be converted into CSV format using
// `gocarina/gocsv` to be consumed by Gnosis' CSV app.
type GnosisCSVTransfer struct {
	TokenType    TokenType       `csv:"token_type"`
	TokenAddress EmptyAddress    `csv:"token_address"`
	Receiver     common.Address  `csv:"receiver"`
	Amount       decimal.Decimal `csv:"amount"`
	ID           uint64          `csv:"id"`
}

// NewERC1155Transfer creates a new ERC1155 transfer for Gnosis' CSV app.
func NewERC1155Transfer(collection common.Address, id uint64, amount uint32, receiver common.Address) *GnosisCSVTransfer {
	return &GnosisCSVTransfer{
		TokenType:    ERC721,
		TokenAddress: EmptyAddress(collection),
		Receiver:     receiver,
		Amount:       decimal.NewFromInt32(int32(amount)),
		ID:           id,
	}
}

// NewERC721Transfer creates a new ERC721 transfer for Gnosis' CSV app.
func NewERC721Transfer(collection common.Address, id uint64, receiver common.Address) *GnosisCSVTransfer {
	return &GnosisCSVTransfer{
		TokenType:    ERC721,
		TokenAddress: EmptyAddress(collection),
		Receiver:     receiver,
		ID:           id,
	}
}

// NewERC20Transfer creates a new ERC20 transfer for Gnosis' CSV app.
func NewERC20Transfer(coin common.Address, amount decimal.Decimal, receiver common.Address) *GnosisCSVTransfer {
	return &GnosisCSVTransfer{
		TokenType:    ERC20,
		TokenAddress: EmptyAddress(coin),
		Receiver:     receiver,
		Amount:       amount,
	}
}

// NewETHTransfer creates a new ETH transfer for Gnosis' CSV app.
func NewETHTransfer(amount decimal.Decimal, receiver common.Address) *GnosisCSVTransfer {
	return &GnosisCSVTransfer{
		TokenType: ETH,
		Receiver:  receiver,
		Amount:    amount,
	}
}
