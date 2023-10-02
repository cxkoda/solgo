package eth

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

// A NullableAddress contains a common.Address that can be flagged as being
// Valid or not (i.e. null). Unlike a pointer, a NullableAddress can be reliably
// marshalled to and from JSON and CSV. The zero value is equivalent to null.
type NullableAddress struct {
	common.Address
	Valid bool
}

func (n *NullableAddress) isValid() bool               { return n.Valid }
func (n *NullableAddress) setPayload(a common.Address) { n.Address = a }
func (n *NullableAddress) setValid(to bool)            { n.Valid = to }

// A NullableUint256 contains a uint256.Int that can be flagged as being Valid
// or not (i.e. null). Unlike a pointer, a NullableUint256 can be reliably
// marshalled to and from JSON and CSV. The zero value is equivalent to null.
type NullableUint256 struct {
	uint256.Int
	Valid bool
}

func (n *NullableUint256) isValid() bool            { return n.Valid }
func (n *NullableUint256) setPayload(i uint256.Int) { n.Int = i }
func (n *NullableUint256) setValid(to bool)         { n.Valid = to }

// MarshalJSON marshals the Address to JSON. It returns []byte("null"), nil if
// Null (i.e. explicit JSON null). A non-Null marshalled value is a hex string.
func (n NullableAddress) MarshalJSON() ([]byte, error) {
	return marshalNullable(
		&n,
		func(s string) ([]byte, error) {
			return json.Marshal(s)
		},
		[]byte("null"),
	)
}

// MarshalJSON marshals the Int to JSON. It returns []byte("null"), nil if Null
// (i.e. explicit JSON null). A non-Null marshalled value is a hex string.
func (n NullableUint256) MarshalJSON() ([]byte, error) {
	return marshalNullable(
		&n,
		func(s string) ([]byte, error) {
			return json.Marshal(s)
		},
		[]byte("null"),
	)
}

// MarshalCSV marshals the Address to a hex string. It returns ("", nil) if
// Null.
func (n NullableAddress) MarshalCSV() (string, error) {
	return marshalNullable(&n, echo, "")
}

// MarshalCSV marshals the Int to a hex string. It returns ("", nil) if Null.
func (n NullableUint256) MarshalCSV() (string, error) {
	return marshalNullable(&n, echo, "")
}

// A nullableMarshaler is a Nullable<T> that can be marshalled to CSV / JSON.
type nullableMarshaler interface {
	String() string
	isValid() bool
}

// A wireFormat is an output type from and to which a Nullable<T> can be
// (un)marshalled.
type wireFormat interface {
	interface{ string | []byte }
}

func marshalNullable[W wireFormat](n nullableMarshaler, fn func(string) (W, error), empty W) (W, error) {
	if n == nil || !n.isValid() {
		return empty, nil
	}
	return fn(n.String())
}

// UnmarshalJSON unmarshals the Address from JSON.
func (n *NullableAddress) UnmarshalJSON(data []byte) error {
	return unmarshalNullable(n, data, unmarshalJSONToString, addressFromString)
}

// UnmarshalJSON unmarshals the Int from JSON.
func (n *NullableUint256) UnmarshalJSON(data []byte) error {
	return unmarshalNullable(n, data, unmarshalJSONToString, uint256FromString)
}

// UnmarshalCSV unmarshals the Address from a hex string.
func (n *NullableAddress) UnmarshalCSV(s string) error {
	return unmarshalNullable(n, s, echo, addressFromString)
}

// UnmarshalCSV unmarshals the Int from a hex string.
func (n *NullableUint256) UnmarshalCSV(s string) error {
	return unmarshalNullable(n, s, echo, uint256FromString)
}

// A nullable is one of the Nullable<T> types.
type nullable interface {
	NullableAddress | NullableUint256
}

// A nullablePayload can be wrapped in a nullable.
type nullablePayload interface {
	common.Address | uint256.Int
}

// A nullablePtr is a pointer to a nullable, extended to include setters.
type nullablePtr[P nullablePayload, N nullable] interface {
	*N
	setPayload(P)
	setValid(bool)
}

func unmarshalNullable[P nullablePayload, N nullable, PtrN nullablePtr[P, N], W wireFormat](
	n PtrN, data W, wireToString func(W) (string, error), stringToPayload func(string) (P, error),
) error {
	var (
		payload P
		valid   bool
	)

	if len(data) != 0 {
		s, err := wireToString(data)
		if err != nil {
			return err
		}

		p, err := stringToPayload(s)
		if err != nil {
			return err
		}
		payload = p
		valid = true
	}

	n.setPayload(payload)
	n.setValid(valid)
	return nil
}

func unmarshalJSONToString(data []byte) (string, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return "", fmt.Errorf("json.Unmarshal(%q, %T): %v", data, &s, err)
	}
	return s, nil
}

// echo is used by unmarshalling when the input is a string because a converter
// from wire format to string is required.
func echo(s string) (string, error) { return s, nil }

func addressFromString(s string) (common.Address, error) {
	return common.HexToAddress(s), nil
}

func uint256FromString(s string) (uint256.Int, error) {
	u, err := uint256.FromHex(s)
	if err != nil {
		return uint256.Int{}, fmt.Errorf("uint256.FromHex(%q): %v", s, err)
	}
	return *u, nil
}
