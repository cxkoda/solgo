package eth

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/h-fam/errdiff"
	"google.golang.org/protobuf/proto"
)

type validator interface {
	proto.Message
	Validate() error
	ValidateAll() error
}

func TestValidation(t *testing.T) {
	val := func(p isValue_Payload) *Value {
		return &Value{
			Payload: p,
		}
	}

	// Only once bit-length instance of each general type needs to be tested as
	// this demonstrates end-to-end behaviour; the generalisation of the
	// behaviour across the entire type is ensured by tests of the rule
	// extensions on FieldDescriptorProtos.
	tests := []struct {
		msg interface {
			proto.Message
			Validate() error
			ValidateAll() error
		}
		errDiffAgainst interface{}
	}{
		{
			msg:            val(nil),
			errDiffAgainst: "Value.Payload: value is required",
		},
		{
			msg: val(&Value_Address{&Address{Bytes: make([]byte, 20)}}),
		},
		{
			msg: val(&Value_Address{}),
		},
		{
			msg: func() *Value {
				a := common.HexToAddress("0xdead")
				v, err := AsValue(a)
				if err != nil {
					t.Fatalf("Bad test setup; AsValue(%T(%+v)) error %v", a, a, err)
				}
				return v
			}(),
		},
		{
			msg:            val(&Value_Address{&Address{Bytes: make([]byte, 21)}}),
			errDiffAgainst: "at most 20 bytes",
		},
		{
			msg: val(&Value_Uint256{Uint256: make([]byte, 30)}),
		},
		{
			msg: val(&Value_Uint256{Uint256: make([]byte, 31)}),
		},
		{
			msg: val(&Value_Uint256{Uint256: make([]byte, 32)}),
		},
		{
			msg: val(&Value_Uint256{Uint256: func() []byte {
				max := new(big.Int).Lsh(big.NewInt(1), 256)
				max.Sub(max, big.NewInt(1))
				return max.Bytes()
			}()}),
		},
		{
			msg:            val(&Value_Uint256{Uint256: make([]byte, 33)}),
			errDiffAgainst: "at most 32 bytes",
		},
		{
			msg: val(&Value_Uint32{Uint32: math.MaxUint32}),
		},
		{
			msg:            val(&Value_Uint32{Uint32: math.MaxUint32 + 1}),
			errDiffAgainst: fmt.Sprintf("less than or equal to %d", math.MaxUint32),
		},
		{
			msg: val(&Value_Int32{Int32: math.MinInt32}),
		},
		{
			msg: val(&Value_Int32{Int32: math.MaxInt32}),
		},
		{
			msg:            val(&Value_Int32{Int32: math.MinInt32 - 1}),
			errDiffAgainst: fmt.Sprintf("inside range [%d, %d]", math.MinInt32, math.MaxInt32),
		},
		{
			msg:            val(&Value_Int32{Int32: math.MaxInt32 + 1}),
			errDiffAgainst: fmt.Sprintf("inside range [%d, %d]", math.MinInt32, math.MaxInt32),
		},
	}

	for _, tt := range tests {
		if diff := errdiff.Check(tt.msg.Validate(), tt.errDiffAgainst); diff != "" {
			t.Errorf("%T(%+v).Validate() %s", tt.msg, tt.msg, diff)
		}
	}
}
