// Package flagtype exposes custom flag types, compatible with both the flag and
// pflag packages.
package flagtype

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// A StringSet is a set of strings that accepts comma-separated commandline
// values, similar to pflag.StringSlice(). The empty string on the command line
// results in the empty set, not in {""}.
type StringSet map[string]struct{}

// NewStringSet returns a fresh StringSet with the specified elements.
func NewStringSet(vals ...string) StringSet {
	s := make(StringSet, len(vals))
	for _, v := range vals {
		s[v] = struct{}{}
	}
	return s
}

// String returns the StringSet values as a comma-separated string.
func (s StringSet) String() string {
	vals := make([]string, 0, len(s))
	for v := range s {
		vals = append(vals, v)
	}
	sort.Strings(vals)
	return strings.Join(vals, ",")
}

// Set clears all elements in the StringSet, splits `raw` by comma, and inserts
// all resulting parts as values in the set. If raw == "" then s becomes the
// empty set.
func (s StringSet) Set(raw string) error {
	for v := range s {
		delete(s, v)
	}
	if raw == "" {
		return nil
	}

	for _, v := range strings.Split(raw, ",") {
		s[v] = struct{}{}
	}
	return nil
}

// Type returns the fully qualified type of s.
func (s StringSet) Type() string {
	return fmt.Sprintf("%T", s)
}

// A StringToStringSet is a map of string sets, themselves keyed by strings.
// Each set is a comma-separated list of strings (possibly empty), prefixed by
// the set's key and an = symbol. Sets are delimited by semi-colons. For
// example:
//
//	Input: foo=a,b,c;bar=d,e,f;baz=
//	Result: {foo: [a,b,c], bar: [d,e,f], baz: []}
//
// The empty string on the command line results in an empty map, not in {"":
// []}.
type StringToStringSet map[string]StringSet

func (s StringToStringSet) String() string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s=%s", k, s[k].String())
	}
	return strings.Join(parts, ";")
}

// Set clears all the elements in the StringToStringSet, splits raw by
// semi-colon, and inserts all resulting comma-separated lists as per StringSet.
// If raw == "" then s becomes the empty map.
func (s StringToStringSet) Set(raw string) error {
	for v := range s {
		delete(s, v)
	}
	if raw == "" {
		return nil
	}

	for _, set := range strings.Split(raw, ";") {
		kv := strings.SplitN(set, "=", 2)
		k, v := kv[0], kv[1]

		s[k] = make(StringSet)
		if err := s[k].Set(v); err != nil {
			return err
		}
	}
	return nil
}

// Type returns the fully qualified type of s.
func (s StringToStringSet) Type() string {
	return fmt.Sprintf("%T", s)
}

// An ETHAddress is a go-ethereum Address.
type ETHAddress struct {
	common.Address
}

// Set parses the raw hex string as an ETH address. The 0x prefix is optional.
func (a *ETHAddress) Set(raw string) error {
	if !common.IsHexAddress(raw) {
		return notETHAddressErr(raw)
	}
	a.Address = common.HexToAddress(raw)
	return nil
}

func notETHAddressErr(raw string) error {
	return fmt.Errorf("%q not an ETH address", raw)
}

// Type returns the fully qualified type of a.
func (a *ETHAddress) Type() string {
	return fmt.Sprintf("%T", a)
}

// An ETHAddressSlice is a go-ethereum Address (parsed as ETHAddresses).
type ETHAddressSlice []common.Address

// Set parses the raw, comma-separated hex strings as ETH addresses. The 0x prefix is optional.
func (as *ETHAddressSlice) Set(raw string) error {
	if raw == "" {
		return nil
	}

	for _, v := range strings.Split(raw, ",") {
		var a ETHAddress
		if err := a.Set(v); err != nil {
			return err
		}
		*as = append(*as, a.Address)
	}

	return nil
}

// Type returns the fully qualified type of a.
func (a *ETHAddressSlice) Type() string {
	return fmt.Sprintf("%T", a)
}

// String returns the ETHAddressSlice values as a comma-separated string.
func (as ETHAddressSlice) String() string {
	vals := make([]string, 0, len(as))
	for _, v := range as {
		vals = append(vals, v.String())
	}
	return strings.Join(vals, ",")
}

// A Date represents an ISO8601 date in the format YYYY-MM-DD. The resulting
// time is midnight UTC on the date; i.e. the beginning of the day:
// YYYY-MM-DDT00:00:00Z.
type Date time.Time

const dateLayout = "2006-01-02"

// Set parses the raw string as YYYY-MM-DD.
func (d *Date) Set(raw string) error {
	t, err := time.Parse(dateLayout, raw)
	if err != nil {
		return fmt.Errorf("time.Parse(%q, %q): %v", dateLayout, raw, err)
	}
	*d = Date(t)
	return nil
}

// String returns the date in the format YYYY-MM-DD. If d is nil, the return
// value is undefined.
func (d *Date) String() string {
	return d.AsTime().Format(dateLayout)
}

// Type returns the fully qualified type of d.
func (d *Date) Type() string {
	return fmt.Sprintf("%T", d)
}

// AsTime returns d as a time.Time. If d is nil, a zero Time value is returned.
func (d *Date) AsTime() time.Time {
	var t time.Time
	if d != nil {
		t = time.Time(*d)
	}
	return t
}
