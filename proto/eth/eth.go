package eth

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// The type support in this file is likely incomplete and will be extended on an
// as-needed basis.

// Parse returns v.Payload as a native Go or go-ethereum type where unambiguous
// otherwise it returns it unmodified.
func (v *Value) Parse() (interface{}, error) {
	if err := v.ValidateAll(); err != nil {
		return nil, err
	}
	return v.MustParseWithoutValidation(), nil
}

// MustParseWithoutValidation is identical to Parse except it doesn't call
// v.Validate() before parsing.
//
// Behaviour is undefined (including panic, truncation, invalid data, etc) when
// validation would have otherwise failed. This function MUST only be used if
// validation has already been performed elsewhere.
func (v *Value) MustParseWithoutValidation() interface{} {
	switch p := v.Payload.(type) {
	case nil:
		return nil

	case *Value_Address:
		return common.BytesToAddress(p.Address.Bytes)
	case *Value_Bool:
		return p.Bool
	case *Value_String_:
		return p.String_

	case *Value_Bytes32:
		var b [32]byte
		copy(b[:], p.Bytes32)
		return b

	case *Value_Uint256:
		u, overflow := uint256.FromBig(new(big.Int).SetBytes(p.Uint256))
		if overflow {
			panic(fmt.Sprintf("%T %#x overflowed when converting to %T", p, p.Uint256, u))
		}
		return u

	default:
		return p
	}
}

// AsValue returns p as a Value only if the mapping to Payload type is
// unambigous.
func AsValue(p interface{}) (*Value, error) {
	switch p := p.(type) {
	case common.Address:
		return value(&Value_Address{Address: &Address{Bytes: p.Bytes()}}), nil
	case bool:
		return value(&Value_Bool{Bool: p}), nil
	case string:
		return value(&Value_String_{String_: p}), nil

	case [32]byte:
		return value(&Value_Bytes32{Bytes32: p[:]}), nil

	case *uint256.Int:
		return value(&Value_Uint256{Uint256: p.ToBig().Bytes()}), nil

	case uint8:
		return value(&Value_Uint8{Uint8: uint64(p)}), nil

	default:
		return nil, fmt.Errorf("%T cannot be unambiguously mapped to a payload type", p)
	}
}

func value(p isValue_Payload) *Value {
	return &Value{Payload: p}
}

// ErrSetNilPayload is returned by Value.SetPayload() when the existing payload
// is nil because the existing value is used as a type hint.
var ErrSetNilPayload = errors.New("Value.SetPayload() with nil existing payload")

// SetPayload inspects the current type of v.Payload and uses it as a type hint
// for setting the payload value.
func (v *Value) SetPayload(to interface{}) error {
	switch p := v.Payload.(type) {
	case nil:
		return ErrSetNilPayload

	case *Value_Address:
		switch addr := to.(type) {
		case common.Address:
			p.Address = &Address{Bytes: addr.Bytes()}
			return nil

		case []byte:
			p.Address = &Address{Bytes: common.LeftPadBytes(addr, 20)}
			return v.Validate()
		}

	case *Value_Bool:
		if b, ok := to.(bool); ok {
			p.Bool = b
			return nil
		}

	case *Value_Bytes:
		if b, ok := to.([]byte); ok {
			p.Bytes = b
			return nil
		}

	case *Value_String_:
		if s, ok := to.(string); ok {
			p.String_ = s
			return nil
		}

	case *Value_Uint8:
		if u, ok := to.(uint8); ok {
			p.Uint8 = uint64(u)
			return nil
		}

	case *Value_Uint256:
		switch val := to.(type) {
		case *uint256.Int:
			p.Uint256 = val.Bytes()
			return nil

		case *big.Int:
			p.Uint256 = val.Bytes()
			return v.Validate()
		}
	}

	return fmt.Errorf("cannot set %T to %T; some valid conversions may yet to be implemented", v.Payload, to)
}

// Time returns b.TimeStamp.AsTime().
func (b *Block) Time() time.Time {
	return b.TimeStamp.AsTime()
}

// valuePayloadOneofDescriptor is the oneof descriptor for Value.payload.
var valuePayloadOneofDescriptor protoreflect.OneofDescriptor

func init() {
	v := &Value{}
	valuePayloadOneofDescriptor = v.ProtoReflect().Descriptor().Oneofs().ByName("payload")
}

// NewEvent constructs a new Event, populating both Arguments and
// ArgumentsByName with the same values.
func NewEvent(name string, emitter common.Address, args ...*Argument) *Event {
	ev := &Event{
		Name:            name,
		Arguments:       args,
		ArgumentsByName: make(map[string]*Argument),
		Emitter:         &Address{Bytes: emitter.Bytes()},
	}
	for _, a := range ev.Arguments {
		ev.ArgumentsByName[a.Name] = a
	}
	return ev
}

// NewArgument constructs a new Argument with its type defined by the Payload.
func NewArgument(name string, typ isValue_Payload, indexed bool) *Argument {
	return &Argument{
		Name:    name,
		Value:   &Value{Payload: typ},
		Indexed: indexed,
	}
}

// EVMString returns the Event in the standard Ethereum VM string format.
func (ev *Event) EVMString() string {
	return fmt.Sprintf("%s(%s)", ev.Name, strings.Join(ev.params(), ","))
}

func (ev *Event) params() []string {
	params := make([]string, len(ev.Arguments))
	for i, arg := range ev.Arguments {
		fld := arg.Value.ProtoReflect().WhichOneof(valuePayloadOneofDescriptor)
		params[i] = string(fld.Name())
	}
	return params
}

// EVMHash returns the identifier hash of the Event; i.e. sha3(EVMString()).
func (ev *Event) EVMHash() common.Hash {
	return crypto.Keccak256Hash([]byte(ev.EVMString()))
}

// ABIArguments returns the Event's Arguments as go-ethereum equivalents,
// usually for packing/unpacking of data and topics.
func (ev *Event) ABIArguments() (abi.Arguments, error) {
	args := make(abi.Arguments, len(ev.Arguments))
	params := ev.params()

	for i, arg := range ev.Arguments {
		t, err := abi.NewType(params[i], "", nil)
		if err != nil {
			return nil, fmt.Errorf(`abi.NewType(%q, "", nil): %v`, params[i], err)
		}
		args[i] = abi.Argument{
			Name:    arg.Name,
			Type:    t,
			Indexed: arg.Indexed,
		}
	}

	return args, nil
}
