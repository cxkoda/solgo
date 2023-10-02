package eth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gocarina/gocsv"
	"github.com/google/go-cmp/cmp"
	"github.com/holiman/uint256"
)

// Assert that Nullable<T> can be marshalled to and from JSON and CSV.
var _ = []interface {
	json.Marshaler
	json.Unmarshaler
	gocsv.TypeMarshaller
	gocsv.TypeUnmarshaller
}{
	&NullableAddress{},
	&NullableUint256{},
}

// nullableTestData carries data for marshalling to and from CSV/JSON.
type nullableTestData[P nullablePayload, N nullable, PtrN nullablePtr[P, N]] struct {
	// X is always set to the value "x" to demonstrate (un)marshalling in
	// context, especially when the Nullable results in empty output.
	X        string `json:"x"`
	Nullable PtrN   `json:"nullable"`
}

type nullableTestCase[P nullablePayload, N nullable, PtrN nullablePtr[P, N]] struct {
	payload           P
	valid, nilPointer bool
	wantJSON, wantCSV string
}

func (tt *nullableTestCase[P, N, PtrN]) run(t *testing.T) {
	t.Run(fmt.Sprintf("%#x valid=%t nil-pointer=%t", tt.payload, tt.valid, tt.nilPointer), func(t *testing.T) {
		// Is there a simpler way to achieve new(T)?
		var n N
		nn := PtrN(&n)
		nn.setPayload(tt.payload)
		nn.setValid(tt.valid)

		in := nullableTestData[P, N, PtrN]{
			X:        "x",
			Nullable: &n,
		}
		if tt.nilPointer {
			in.Nullable = nil
		}

		t.Run("MarshalJSON", func(t *testing.T) {
			got, err := json.Marshal(in)
			if err != nil || string(got) != tt.wantJSON {
				t.Fatalf("json.Marshal(%T{%+v}) got %q, err = %v; want %q, nil err", in, in, got, err, tt.wantJSON)
			}
		})

		t.Run("UnmarshalJSON", func(t *testing.T) {
			var got nullableTestData[P, N, PtrN]
			if err := json.Unmarshal([]byte(tt.wantJSON), &got); err != nil {
				t.Fatalf("json.Unmarshal(%q, %T) error %v", tt.wantJSON, &got, err)
			}

			want := nullableTestData[P, N, PtrN]{X: "x"}
			if !tt.nilPointer && tt.valid {
				want.Nullable = in.Nullable
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("After json.Unmarshal(%q, %T); diff (-want +got):\n%s", tt.wantJSON, &got, diff)
			}
		})

		t.Run("MarshalCSV", func(t *testing.T) {
			var got bytes.Buffer
			if err := gocsv.Marshal([]nullableTestData[P, N, PtrN]{in}, &got); err != nil {
				t.Fatalf("gocsv.Marshal(%T{%+v}, …) error %v", in, in, err)
			}
			if got := got.String(); got != tt.wantCSV {
				t.Errorf("gocsv.Marshal(%T{%+v)},…) got %q; want %q", in, in, got, tt.wantCSV)
			}
		})

		t.Run("UnmarshalCSV", func(t *testing.T) {
			var got []nullableTestData[P, N, PtrN]
			if err := gocsv.Unmarshal(strings.NewReader(tt.wantCSV), &got); err != nil {
				t.Fatalf("gocsv.Unmarshal(%q, %T) error %v", tt.wantCSV, &got, err)
			}

			var n N
			want := []nullableTestData[P, N, PtrN]{{
				X:        "x",
				Nullable: &n,
			}}
			if !tt.nilPointer && tt.valid {
				want[0].Nullable.setValid(true)
				want[0].Nullable.setPayload(tt.payload)
			}

			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("After gocsv.Unmarshal(%q, %T); diff (-want +got):\n%s", tt.wantCSV, &got, diff)
			}
		})
	})
}

func TestNullableAddress(t *testing.T) {
	var zero common.Address
	nonZero := common.HexToAddress("0xc0ffee")
	const withCheckSum = "0x4e03D41D6a46698A8AFF444a553fd5020d9BC368"

	for _, tt := range []nullableTestCase[common.Address, NullableAddress, *NullableAddress]{
		{
			payload:  zero,
			valid:    true,
			wantJSON: `{"x":"x","nullable":"` + zero.Hex() + `"}`,
			wantCSV:  "X,Nullable\nx," + zero.Hex() + "\n",
		},
		{
			payload:  zero,
			valid:    false,
			wantJSON: `{"x":"x","nullable":null}`,
			wantCSV:  "X,Nullable\nx,\n",
		},
		{
			nilPointer: true,
			wantJSON:   `{"x":"x","nullable":null}`,
			wantCSV:    "X,Nullable\nx,\n",
		},
		{
			payload:  nonZero,
			valid:    true,
			wantJSON: `{"x":"x","nullable":"` + nonZero.Hex() + `"}`,
			wantCSV:  "X,Nullable\nx," + nonZero.Hex() + "\n",
		},
		{
			payload:  nonZero, // ignore corrupted address when still Null
			valid:    false,
			wantJSON: `{"x":"x","nullable":null}`,
			wantCSV:  "X,Nullable\nx,\n",
		},
		{
			payload:  common.HexToAddress(withCheckSum),
			valid:    true,
			wantJSON: `{"x":"x","nullable":"` + withCheckSum + `"}`,
			wantCSV:  "X,Nullable\nx," + withCheckSum + "\n",
		},
	} {
		tt.run(t)
	}
}

func TestNullableUint256(t *testing.T) {
	var zero uint256.Int
	nonZero := *(uint256.NewInt(123456789))

	const coffee = "0xdecafc0ffee"
	coffeeInt, err := uint256.FromHex(coffee)
	if err != nil {
		t.Fatalf("uint256.FromHex(%q) error %v", coffee, err)
	}

	for _, tt := range []nullableTestCase[uint256.Int, NullableUint256, *NullableUint256]{
		{
			payload:  zero,
			valid:    true,
			wantJSON: `{"x":"x","nullable":"` + zero.Hex() + `"}`,
			wantCSV:  "X,Nullable\nx," + zero.Hex() + "\n",
		},
		{
			payload:  zero,
			valid:    false,
			wantJSON: `{"x":"x","nullable":null}`,
			wantCSV:  "X,Nullable\nx,\n",
		},
		{
			nilPointer: true,
			wantJSON:   `{"x":"x","nullable":null}`,
			wantCSV:    "X,Nullable\nx,\n",
		},
		{
			payload:  nonZero,
			valid:    true,
			wantJSON: `{"x":"x","nullable":"` + nonZero.Hex() + `"}`,
			wantCSV:  "X,Nullable\nx," + nonZero.Hex() + "\n",
		},
		{
			payload:  nonZero, // ignore corrupted address when still Null
			valid:    false,
			wantJSON: `{"x":"x","nullable":null}`,
			wantCSV:  "X,Nullable\nx,\n",
		},
		{
			payload:  *coffeeInt,
			valid:    true,
			wantJSON: `{"x":"x","nullable":"` + coffee + `"}`,
			wantCSV:  "X,Nullable\nx," + coffee + "\n",
		},
	} {
		tt.run(t)
	}
}
