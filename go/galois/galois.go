// Package galois provides functionality over Galois (finite) fields,
// implemented over the integers modulo n.
//
// Operations do NOT run in cryptographic constant time.
package galois

import (
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
)

var (
	bigZero = big.NewInt(0)
	bigOne  = big.NewInt(1)
)

// A Field represents a finite field of specific order.
type Field big.Int

// NewField returns a new Field of the specified order.
func NewField(order *big.Int) *Field {
	return (*Field)(order)
}

// Order returns the order of the Field.
func (f *Field) Order() *big.Int {
	return new(big.Int).Set((*big.Int)(f))
}

// Exp returns x**y mod f.Order().
func (f *Field) Exp(x, y *big.Int) *big.Int {
	return new(big.Int).Exp(x, y, f.Order())
}

// Mul returns x*y mod f.Order().
func (f *Field) Mul(x, y *big.Int) *big.Int {
	p := new(big.Int).Mul(x, y)
	return p.Mod(p, f.Order())
}

// MultInverse returns the multiplicative inverse of x.
func (f *Field) MultInverse(x *big.Int) *big.Int {
	return new(big.Int).ModInverse(x, f.Order())
}

// Random returns a random field element from [0,q). The Reader is propagated to
// rand.Int().
func (f *Field) Random(r io.Reader) (*big.Int, error) {
	x, err := rand.Int(r, f.Order())
	if err != nil {
		return nil, fmt.Errorf("rand.Int(): %v", err)
	}
	return x, nil
}

// RootOfUnity returns a random nth root of unity; n must be even. A primitive
// root is one that can be used to generate all corresponding roots by raising
// it to each of [0,n). If n does not divide (q-1), the only root is 1 itself,
// which is returned (regardless of the value of `primitive`).
//
// The io.Reader is used to choose a random field element from which a root is
// determined via crypto/rand.Int(). If primitive==false this will only be
// called once, but primitive roots are determined probabilistically with 50%
// success rate on each attempt.
//
// As we're in a cyclic group, for any non-zero x: x^q = x, so x^(q-1) = 1.
// Since x^(y/n)^n = x^y, x^((q-1)/n) is an nth root of 1 mod q. Detection of a
// primitive root is described in https://crypto.stackexchange.com/a/63616.
func (f *Field) RootOfUnity(r io.Reader, n uint64, primitive bool) (*big.Int, error) {
	if n%2 == 1 {
		return nil, fmt.Errorf("can only calculate even roots of unity; n = %d", n)
	}

	bigN := new(big.Int).SetUint64(n)

	qSub1 := new(big.Int).Sub(f.Order(), bigOne)
	qSub1OverN, rem := new(big.Int).DivMod(qSub1, bigN, new(big.Int))
	if rem.Cmp(bigZero) != 0 {
		return big.NewInt(1), nil
	}

	halfN := new(big.Int).Rsh(bigN, 1)
	var root *big.Int
	for {
		x, err := f.Random(r)
		if err != nil {
			return nil, err
		}
		root = f.Exp(x, qSub1OverN)
		if !primitive || f.Exp(root, halfN).Cmp(bigOne) != 0 {
			return root, nil
		}
	}
}
