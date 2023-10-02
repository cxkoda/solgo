package eth

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/go-cmp/cmp"
	"github.com/h-fam/errdiff"
	"github.com/holiman/uint256"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	validpb "github.com/envoyproxy/protoc-gen-validate/validate"
	descpb "google.golang.org/protobuf/types/descriptorpb"

	_ "embed"
)

//go:embed eth-descriptor-set.bin
var descriptorSetBin []byte

func TestValuePayloadInvariants(t *testing.T) {
	// Although this test is merely a change detector, it's important to ensure
	// that all invariants are in place and there are >=100 fields.

	files := new(descpb.FileDescriptorSet)
	if err := proto.Unmarshal(descriptorSetBin, files); err != nil {
		t.Fatalf("proto.Unmarshal([embeded descriptor binary], %T) error %v", files, err)
	}

	got := valueMessage(t, files).Field

	t.Run("all lower case", func(t *testing.T) {
		// Solidity signatures are based on the lower-case normalised naming of
		// types.
		for _, fld := range got {
			if n := fld.GetName(); n != strings.ToLower(n) {
				t.Errorf("Field name %q must be lower-case", n)
			}
		}
	})

	// fld builds and returns a FieldDescriptorProto
	fld := func(name string, num int32, typ descpb.FieldDescriptorProto_Type) *descpb.FieldDescriptorProto {
		return &descpb.FieldDescriptorProto{
			Name:       proto.String(name),
			Number:     proto.Int32(num),
			Type:       typ.Enum(),
			Label:      descpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			OneofIndex: proto.Int32(0),
			JsonName:   proto.String(name),
		}
	}
	const (
		tBool   = descpb.FieldDescriptorProto_TYPE_BOOL
		tBytes  = descpb.FieldDescriptorProto_TYPE_BYTES
		tMsg    = descpb.FieldDescriptorProto_TYPE_MESSAGE
		tString = descpb.FieldDescriptorProto_TYPE_STRING
	)
	want := []*descpb.FieldDescriptorProto{
		fld("address", 1, tMsg),
		fld("bool", 2, tBool),
		fld("bytes", 3, tBytes),
		fld("string", 4, tString),
	}
	want[0].TypeName = proto.String(".proof.eth.Address")

	// bytes1 to bytes32
	for i := 1; i <= 32; i++ {
		f := fld(fmt.Sprintf("bytes%d", i), int32(i+4), tBytes)
		setValidationRules(t, f, &validpb.FieldRules{
			Type: &validpb.FieldRules_Bytes{Bytes: &validpb.BytesRules{
				Len:         proto.Uint64(uint64(i)),
				IgnoreEmpty: proto.Bool(true),
			}},
		})
		want = append(want, f)
	}

	// signed integers
	for i := 1; i <= 32; i++ {
		bits := i * 8
		f := fld(fmt.Sprintf("int%d", bits), int32(i+36), tBytes)

		switch {
		case bits < 64:
			lim := int64(1 << (bits - 1))
			setValidationRules(t, f, &validpb.FieldRules{
				Type: &validpb.FieldRules_Int64{Int64: &validpb.Int64Rules{
					Gte: proto.Int64(-lim),
					Lte: proto.Int64(lim - 1),
				}},
			})

			fallthrough
		case bits <= 64:
			f.Type = descpb.FieldDescriptorProto_TYPE_INT64.Enum()

		default:
			setValidationRules(t, f, &validpb.FieldRules{
				Type: &validpb.FieldRules_Bytes{Bytes: &validpb.BytesRules{
					Len:         proto.Uint64(uint64(i)),
					IgnoreEmpty: proto.Bool(true),
				}},
			})
		}

		want = append(want, f)
	}

	// unsigned integers
	for i := 1; i <= 32; i++ {
		bits := i * 8
		f := fld(fmt.Sprintf("uint%d", bits), int32(i+68), tBytes)

		switch {
		case bits < 64:
			setValidationRules(t, f, &validpb.FieldRules{
				Type: &validpb.FieldRules_Uint64{Uint64: &validpb.UInt64Rules{
					Lte: proto.Uint64((1 << bits) - 1),
				}},
			})

			fallthrough
		case bits <= 64:
			f.Type = descpb.FieldDescriptorProto_TYPE_UINT64.Enum()

		default:
			setValidationRules(t, f, &validpb.FieldRules{
				Type: &validpb.FieldRules_Bytes{Bytes: &validpb.BytesRules{
					MaxLen:      proto.Uint64(uint64(i)),
					IgnoreEmpty: proto.Bool(true),
				}},
			})
		}

		want = append(want, f)
	}

	if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
		t.Errorf("Value.payload message fields diff (-want +got):\n%s", diff)
	}
}

// valueMessage returns the Value message's descriptor.
func valueMessage(tb testing.TB, files *descpb.FileDescriptorSet) *descpb.DescriptorProto {
	tb.Helper()

	for _, file := range files.File {
		for _, msg := range file.MessageType {
			if msg.GetName() == "Value" {
				return msg
			}
		}
	}

	tb.Fatal("Value message not found")
	return nil
}

// setValidationRules sets the protoc-gen-proto E_Rules extension on fld.
func setValidationRules(tb testing.TB, fld *descpb.FieldDescriptorProto, rules *validpb.FieldRules) {
	tb.Helper()
	defer func() {
		if r := recover(); r != nil {
			tb.Fatalf("proto.SetExtension(): %v", r)
		}
	}()

	if fld.Options == nil {
		fld.Options = &descpb.FieldOptions{}
	}
	proto.SetExtension(fld.Options, validpb.E_Rules, rules)
}

func TestPayloadConversion(t *testing.T) {
	leftPad := common.LeftPadBytes
	rightPad := common.RightPadBytes

	tests := []struct {
		native interface{}
		pb     isValue_Payload
	}{
		{
			native: common.HexToAddress("01234567"),
			pb: &Value_Address{
				Address: &Address{Bytes: leftPad([]byte{0x01, 0x23, 0x45, 0x67}, 20)},
			},
		},
		{
			native: common.HexToAddress("deadbeef"),
			pb: &Value_Address{
				Address: &Address{Bytes: leftPad([]byte{0xde, 0xad, 0xbe, 0xef}, 20)},
			},
		},
		{
			native: true,
			pb:     &Value_Bool{Bool: true},
		},
		{
			native: false,
			pb:     &Value_Bool{Bool: false},
		},
		{
			native: "",
			pb:     &Value_String_{String_: ""},
		},
		{
			native: "foo",
			pb:     &Value_String_{String_: "foo"},
		},
		{
			native: [32]byte{},
			pb:     &Value_Bytes32{Bytes32: leftPad(nil, 32)},
		},
		{
			native: [32]byte{42},
			pb:     &Value_Bytes32{Bytes32: rightPad([]byte{42}, 32)},
		},
		{
			native: uint256.NewInt(0),
			pb:     &Value_Uint256{Uint256: []byte{}},
		},
		{
			native: uint256.NewInt(42),
			pb:     &Value_Uint256{Uint256: []byte{42}},
		},
		{
			native: func() *uint256.Int {
				const hex = "0xdeadbeef0123456789abcdef"
				u, err := uint256.FromHex(hex)
				if err != nil {
					t.Fatalf("Bad test setup; uint256.FromHex(%q) error %v", hex, err)
				}
				return u
			}(),
			pb: &Value_Uint256{Uint256: []byte{
				0xde, 0xad, 0xbe, 0xef,
				0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%T", tt.pb), func(t *testing.T) {
			v := value(tt.pb)

			t.Run("Parse", func(t *testing.T) {
				got, err := v.Parse()
				if err != nil {
					t.Fatalf("%T(%v).Parse() error %v", v, v, err)
				}
				if want := tt.native; !cmp.Equal(got, want) {
					t.Errorf("%T(%v).Parse() got %T(%v) want %T(%v)", v, v, got, got, want, want)
				}
			})

			t.Run("AsValue", func(t *testing.T) {
				got, err := AsValue(tt.native)
				if err != nil {
					t.Fatalf("AsValue(%T(%v)) error %v", tt.native, tt.native, err)
				}

				want := &Value{Payload: tt.pb}
				if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
					t.Errorf("AsValue(%T(%v)) diff (-want +got):\n%s", tt.native, tt.native, diff)
				}
			})
		})
	}
}

func TestSetPayload(t *testing.T) {
	tests := []struct {
		val            *Value
		setTo          interface{}
		errDiffAgainst interface{}
		want           *Value
	}{
		{
			val:   value(&Value_Address{}),
			setTo: common.HexToAddress("0123"),
			want: value(&Value_Address{
				Address: &Address{Bytes: common.LeftPadBytes([]byte{0x01, 0x23}, 20)},
			}),
		},
		{
			val:   value(&Value_Address{}),
			setTo: []byte{0x42},
			want: value(&Value_Address{
				Address: &Address{Bytes: common.LeftPadBytes([]byte{0x42}, 20)},
			}),
		},
		{
			val:            value(&Value_Address{}),
			setTo:          make([]byte, 21),
			errDiffAgainst: "at most 20 bytes",
		},
		{
			val:   value(&Value_Uint256{}),
			setTo: uint256.NewInt(0x9876),
			want:  value(&Value_Uint256{Uint256: []byte{0x98, 0x76}}),
		},
		{
			val:   value(&Value_Uint256{}),
			setTo: big.NewInt(0x420042),
			want:  value(&Value_Uint256{Uint256: []byte{0x42, 0x00, 0x42}}),
		},
	}

	for _, tt := range tests {
		got := proto.Clone(tt.val).(*Value)
		err := got.SetPayload(tt.setTo)

		if diff := errdiff.Check(err, tt.errDiffAgainst); diff != "" {
			t.Errorf("%T(%+v).SetPayload(%T) %s", tt.val, tt.val, tt.setTo, diff)
			continue
		}
		if tt.errDiffAgainst != nil {
			continue
		}

		if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
			t.Errorf("After %T(%+v).SetPayload(%T(%v)) diff (-want +got):\n%s", tt.val, tt.val, tt.setTo, tt.setTo, diff)
		}
	}
}

func TestSignatureIdentifier(t *testing.T) {
	tests := []struct {
		ev         *Event
		wantString string
		wantHash   common.Hash
	}{
		{
			ev: &Event{
				Name: "Transfer",
				Arguments: []*Argument{
					NewArgument("", &Value_Address{}, false),
					NewArgument("", &Value_Address{}, false),
					NewArgument("", &Value_Uint256{}, false),
				},
			},
			wantString: "Transfer(address,address,uint256)",
			wantHash:   common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"),
		},
		{
			ev: &Event{
				Name: "Nested",
				Arguments: []*Argument{
					NewArgument("", &Value_Uint256{}, false),
				},
			},
			wantString: "Nested(uint256)",
			wantHash:   common.HexToHash("0x84bccedf5fbad5c802864c2d64e4562a610a468ba28173bd7528588e4429eaf7"),
		},
		{
			ev: &Event{
				Name: "Unnested",
				Arguments: []*Argument{
					NewArgument("", &Value_Uint256{}, false),
				},
			},
			wantString: "Unnested(uint256)",
			wantHash:   common.HexToHash("0x657500793744fd287ed8e476832a3cb4b7aa5b931cda10bdc773a301e0e9a831"),
		},
		{
			ev: &Event{
				Name: "Unnested",
				Arguments: []*Argument{
					NewArgument("", &Value_Uint256{}, true), // indexing MUST NOT change anything
				},
			},
			wantString: "Unnested(uint256)",
			wantHash:   common.HexToHash("0x657500793744fd287ed8e476832a3cb4b7aa5b931cda10bdc773a301e0e9a831"),
		},
		// TODO(arran) once rules_sol are in place, generate some test cases
		// directly from solc.
	}

	for _, tt := range tests {
		t.Run(tt.wantString, func(t *testing.T) {
			t.Logf("%T = %+v", tt.ev, tt.ev)
			if got, want := tt.ev.EVMString(), tt.wantString; got != want {
				t.Errorf("%T.EVMString() got %q; want %q", tt.ev, got, want)
			}

			gotHash := tt.ev.EVMHash()
			if diff := cmp.Diff(tt.wantHash, gotHash); diff != "" {
				t.Errorf("%T.EVMHash()\ngot:  %#x\nwant: %#x\ndiff (-want +got):\n%s", tt.ev, gotHash, tt.wantHash, diff)
			}
		})
	}
}
