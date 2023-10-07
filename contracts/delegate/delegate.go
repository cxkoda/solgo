// Package delegate provides bindings for the delegate.cash contracts.
package delegate

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	"github.com/proofxyz/solgo/go/eth"
)

// New returns a new IDelegationRegistry binding using the provided backend. All
// chains have the same deployment address.
//
// In keeping with `abigen` style, New*() only binds existing contracts. Use the
// delegate.cash deployment script if on a new chain as this guarantees a
// matching address.
func New(backend bind.ContractBackend) (*IDelegationRegistry, error) {
	return NewIDelegationRegistry(Address(), backend)
}

// Address returns the multi-chain vanity address used by all delegation
// registries.
func Address() common.Address {
	return common.HexToAddress("0x00000000000076A84feF008CDAbe6409d2FE638B")
}

// A Delegation is a generalised structure capturing all delegation types. When
// converted to a Delegation, all type-specific delegations include at least the
// Vault and Delegate fields. A contract delegation also includes the Contract
// field, and a token delegation also includes the TokenID on top of this.
type Delegation struct {
	Vault    common.Address
	Delegate common.Address
	Contract eth.NullableAddress
	TokenID  eth.NullableUint256
}

// A DelegationBuilder can build a Delegation for a given vault address. It is
// typically a converter from another type.
type DelegationBuilder interface {
	Delegation(vault common.Address) *Delegation
}

// AppendDelegations calls fetch(…, vault), converts all results to *Delegation,
// and appends them to curr, returning the extended slice. The original slice is
// unchanged.
//
// The fetch argument is compatible with the following IDelegationRegistry
// methods without the need to specify T:
//
//	GetAppendableDelegatesForAll()
//	GetContractLevelDelegations()
//	GetTokenLevelDelegations()
func AppendDelegations[T DelegationBuilder](ctx context.Context, curr []*Delegation, fetch func(*bind.CallOpts, common.Address) ([]T, error), vault common.Address) ([]*Delegation, error) {
	raw, err := fetch(&bind.CallOpts{Context: ctx}, vault)
	if err != nil {
		return nil, err
	}

	extra := make([]*Delegation, len(raw))
	for i, r := range raw {
		extra[i] = r.Delegation(vault)
	}
	return append(curr, extra...), nil
}

// A delegatedForAll allows a plain Address to convert itself to a Delegation,
// as with the Delegation() methods added to Contract- and Token-level types.
type delegatedForAll common.Address

func (a delegatedForAll) Delegation(vault common.Address) *Delegation {
	return &Delegation{
		Vault:    vault,
		Delegate: common.Address(a),
	}
}

// GetAppendableDelegatesForAll is identical to GetDelegatesForAll except that
// it returns its values as an internal type compatible with the fetch()
// function of AppendDelegations().
func (r *IDelegationRegistry) GetAppendableDelegatesForAll(opts *bind.CallOpts, vault common.Address) ([]DelegationBuilder, error) {
	addrs, err := r.GetDelegatesForAll(opts, vault)
	if err != nil {
		return nil, err
	}

	ds := make([]DelegationBuilder, len(addrs))
	for i, a := range addrs {
		ds[i] = delegatedForAll(a)
	}
	return ds, nil
}

// Delegation returns the ContractDelegation as a Delegation. All fields except
// TokenID will be populated.
func (d IDelegationRegistryContractDelegation) Delegation(vault common.Address) *Delegation {
	return &Delegation{
		Vault:    vault,
		Delegate: d.Delegate,
		Contract: eth.NullableAddress{Address: d.Contract, Valid: true},
	}
}

// Delegation returns the TokenDelegation as a Delegation. All fields will be
// populated.
func (d IDelegationRegistryTokenDelegation) Delegation(vault common.Address) *Delegation {
	id, overflow := uint256.FromBig(d.TokenId)
	if overflow {
		// Since d.TokenId is generated code, from abigen, if this overflows as
		// uint256 then there are problems geth-wide. We therefore panic to make
		// a cleaner API.
		panic(fmt.Sprintf("uint256.FromBig(%v) overflowed", d.TokenId))
	}
	return &Delegation{
		Vault:    vault,
		Delegate: d.Delegate,
		Contract: eth.NullableAddress{Address: d.Contract, Valid: true},
		TokenID:  eth.NullableUint256{Int: *id, Valid: true},
	}
}
