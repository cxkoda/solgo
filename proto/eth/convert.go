package eth

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

// Convert returns conv(v.Payload) i.f.f. v.Payload is of type P.
func Convert[P isValue_Payload, T any](v *Value, conv func(P) (T, error)) (T, error) {
	var zero T
	p, ok := v.GetPayload().(P)
	if !ok {
		return zero, fmt.Errorf("%T.Payload of type %T; expecting %T", v, v.GetPayload(), p)
	}
	return conv(p)
}

// TODO(arran): generate all of the *Argument methods that are equivalent to
// their underlying Value ones. All they do is decorate the error.

// AsAddress is equivalent to a.Value.AsAddress().
func (a *Argument) AsAddress() (common.Address, error) {
	addr, err := a.GetValue().AsAddress()
	if err != nil {
		return common.Address{}, fmt.Errorf("%T{%q}: %v", a, a.Name, err)
	}
	return addr, nil
}

// AsAddress returns v as a common.Address i.f.f. v.Payload is a *Value_Address.
func (v *Value) AsAddress() (common.Address, error) {
	return Convert(v, func(p *Value_Address) (c common.Address, _ error) {
		copy(c[:], p.Address.Bytes)
		return c, nil
	})
}

// AsUint256 is equivalent to a.Value.AsUint256().
func (a *Argument) AsUint256() (*uint256.Int, error) {
	u, err := a.GetValue().AsUint256()
	if err != nil {
		return nil, fmt.Errorf("%T{%q}: %v", a, a.GetName(), err)
	}
	return u, nil
}

// AsUint256 returns v as a *uint256.Int i.f.f. v.Payload is a *Value_Uint256.
func (v *Value) AsUint256() (*uint256.Int, error) {
	return Convert(v, func(p *Value_Uint256) (*uint256.Int, error) {
		b, err := v.AsBig256()
		if err != nil {
			return nil, err
		}
		u, overflow := uint256.FromBig(b)
		if overflow {
			return nil, fmt.Errorf("0x%s overflows uint256", b.Text(16))
		}
		return u, nil
	})
}

// AsUint256 returns v as a *big.Int i.f.f. v.Payload is a *Value_Uint256.
func (v *Value) AsBig256() (*big.Int, error) {
	return Convert(v, func(p *Value_Uint256) (*big.Int, error) {
		return new(big.Int).SetBytes(p.Uint256), nil
	})
}

// AsUint8 is equivalent to a.Value.AsUint8().
func (a *Argument) AsUint8() (uint8, error) {
	u, err := a.GetValue().AsUint8()
	if err != nil {
		return 0, fmt.Errorf("%T{%q}: %v", a, a.Name, err)
	}
	return u, nil
}

// AsUint8 returns v as a uint8 i.f.f. v.Payload is a *Value_Uint8.
func (v *Value) AsUint8() (uint8, error) {
	return Convert(v, func(p *Value_Uint8) (uint8, error) {
		return uint8(p.Uint8), nil
	})
}

// AsCommon returns h as a common.Hash without performing any validation.
func (h *Hash) AsCommon() (c common.Hash) {
	copy(c[:], h.Bytes)
	return c
}
