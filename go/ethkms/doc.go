// Package ethkms provides KMS-backed EVM addresses.
package ethkms

import (
	"encoding/asn1"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
)

// oidECDSAPubKey is the ASN.1 identifier for ECDSA public keys.
var oidECDSAPubKey = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}

// curveOrder returns the order of the secp256k1 curve.
func curveOrder() *big.Int {
	return new(big.Int).Set(secp256k1.S256().N)
}
