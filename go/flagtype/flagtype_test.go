package flagtype

import (
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/go-cmp/cmp"
	"github.com/h-fam/errdiff"
	"github.com/spf13/pflag"
)

// A valueTest is a test case, compatible with any pflag.Value.
type valueTest[V pflag.Value] struct {
	name string
	// input is any value accepted on the command line whereas canonicalInput is
	// the same effective value as if it were returned by V.String(). If
	// canonicalInput is empty then input is considered to be canonical for the
	// purposes of testing V.String().
	input, canonicalInput string
	want                  V
	// errDiffAgainst is compared against the error returned when setting the
	// flag value via pflag.FlagSet.Set(). If non-nil, the `want` value is not
	// checked.
	errDiffAgainst interface{}
}

// do runs the test. Generics in Go suck so it's not possible to make(V) on map
// types and we must therefore accept an empty value to use as `got`.
func (tt *valueTest[V]) do(t *testing.T, empty V, opts ...cmp.Option) {
	// Deliberately not calling t.Helper() otherwise the error will be reported
	// on the wrong line.

	t.Run(tt.name, func(t *testing.T) {
		t.Run("Set", func(t *testing.T) {
			fSet := pflag.NewFlagSet("testing", pflag.ContinueOnError)

			got := empty
			fSet.Var(got, "x", "")
			if diff := errdiff.Check(fSet.Set("x", tt.input), tt.errDiffAgainst); diff != "" {
				t.Fatalf("%T.Set(%T, %q) %s", fSet, got, tt.input, diff)
			}
			if tt.errDiffAgainst != nil {
				return
			}

			if diff := cmp.Diff(tt.want, got, opts...); diff != "" {
				t.Errorf("After %T.Set(%T, %q) diff (-want +got):\n%s", fSet, got, tt.input, diff)
			}
		})

		if tt.errDiffAgainst != nil {
			return
		}

		t.Run("String", func(t *testing.T) {
			want := tt.canonicalInput
			if want == "" {
				want = tt.input
			}

			if got := tt.want.String(); got != want {
				t.Errorf("%T(%+v).String() got %q; want %q", tt.want, tt.want, got, want)
			}
		})
	})
}

func TestStringSet(t *testing.T) {
	var null struct{}

	tests := []valueTest[StringSet]{
		{
			name:  "empty string = empty set",
			input: "",
			want:  StringSet{},
		},
		{
			name:  "single value",
			input: "a",
			want: StringSet{
				"a": null,
			},
		},
		{
			name:  "different value",
			input: "b",
			want: StringSet{
				"b": null,
			},
		},
		{
			name:  "two values",
			input: "a,b",
			want: StringSet{
				"a": null,
				"b": null,
			},
		},
		{
			name:           "repeat value",
			input:          "a,b,a",
			canonicalInput: "a,b",
			want: StringSet{
				"a": null,
				"b": null,
			},
		},
		{
			name:           "empty value",
			input:          "a,b,",
			canonicalInput: ",a,b",
			want: StringSet{
				"a": null,
				"b": null,
				"":  null,
			},
		},
		{
			name:           "repeated value but with a space is considered different",
			input:          "a,b, a",
			canonicalInput: " a,a,b",
			want: StringSet{
				"a":  null,
				"b":  null,
				" a": null,
			},
		},
		{
			name:           "full words",
			input:          "the,quick,brown,fox",
			canonicalInput: "brown,fox,quick,the",
			want:           NewStringSet("the", "quick", "brown", "fox"),
		},
	}

	for _, tt := range tests {
		tt.do(t, StringSet{})
	}
}

func TestStringToStringSet(t *testing.T) {
	tests := []valueTest[StringToStringSet]{
		{
			name:  "empty string",
			input: "",
			want:  StringToStringSet{},
		},
		{
			name:  "just = symbol",
			input: "=",
			want: StringToStringSet{
				"": {},
			},
		},
		{
			name:  "key without values",
			input: "a=",
			want: StringToStringSet{
				"a": {},
			},
		},
		{
			name:  "key with single value",
			input: "a=b",
			want: StringToStringSet{
				"a": NewStringSet("b"),
			},
		},
		{
			name:  "key with multiple values",
			input: "b=c,d",
			want: StringToStringSet{
				"b": NewStringSet("c", "d"),
			},
		},
		{
			name:           "multiple sets",
			input:          "odd=1,3,5,7;even=2,4,6;empty=;dunno=0",
			canonicalInput: "dunno=0;empty=;even=2,4,6;odd=1,3,5,7",
			want: StringToStringSet{
				"odd":   NewStringSet("1", "3", "5", "7"),
				"even":  NewStringSet("2", "4", "6"),
				"dunno": NewStringSet("0"),
				"empty": {},
			},
		},
	}

	for _, tt := range tests {
		tt.do(t, StringToStringSet{})
	}
}

func TestETHAddress(t *testing.T) {
	const raw = "9999888877776666555544443333222211110000"
	want := &ETHAddress{common.HexToAddress(raw)}

	tests := []valueTest[*ETHAddress]{
		{
			name:           "without 0x prefix",
			input:          raw,
			canonicalInput: "0x" + raw,
			want:           want,
		},
		{
			name:  "with 0x prefix",
			input: "0x" + raw,
			want:  want,
		},
		{
			name:           "too short",
			input:          "0x123456",
			errDiffAgainst: notETHAddressErr("0x123456").Error(),
		},
		{
			name:           "invalid",
			input:          "hello",
			errDiffAgainst: notETHAddressErr("hello").Error(),
		},
	}

	for _, tt := range tests {
		tt.do(t, &ETHAddress{})
	}
}

func TestETHAddressSlice(t *testing.T) {
	const raw1 = "9999888877776666555544443333222211110000"
	const raw2 = "0000111122223333444455556666777788889999"
	wantSingle := &ETHAddressSlice{common.HexToAddress(raw1)}
	wantDouble := &ETHAddressSlice{common.HexToAddress(raw1), common.HexToAddress(raw2)}

	tests := []valueTest[*ETHAddressSlice]{
		{
			name:           "single without 0x prefix",
			input:          raw1,
			canonicalInput: "0x" + raw1,
			want:           wantSingle,
		},
		{
			name:  "single with 0x prefix",
			input: "0x" + raw1,
			want:  wantSingle,
		},
		{
			name:           "double without 0x prefix",
			input:          strings.Join([]string{raw1, raw2}, ","),
			canonicalInput: "0x" + raw1 + ",0x" + raw2,
			want:           wantDouble,
		},
		{
			name:  "double with 0x prefix",
			input: strings.Join([]string{"0x" + raw1, "0x" + raw2}, ","),
			want:  wantDouble,
		},
		{
			name:           "invalid",
			input:          raw1 + ",hello",
			errDiffAgainst: notETHAddressErr("hello").Error(),
		},
	}

	for _, tt := range tests {
		tt.do(t, &ETHAddressSlice{})
	}
}

func TestDate(t *testing.T) {
	mk := func(y int, m time.Month, d int) *Date {
		dt := Date(time.Date(y, m, d, 0, 0, 0, 0, time.UTC))
		return &dt
	}

	tests := []valueTest[*Date]{
		{
			input: "1605-11-05",
			want:  mk(1605, time.November, 5),
		},
		{
			input: "1776-07-04",
			want:  mk(1776, time.July, 4),
		},
	}

	for _, tt := range tests {
		tt.do(t, &Date{}, cmp.Transformer("date2time", func(d *Date) time.Time {
			if d == nil {
				return time.Time{}
			}
			return time.Time(*d)
		}))
	}
}
