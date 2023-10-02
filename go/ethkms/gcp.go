package ethkms

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"google.golang.org/api/option"
)

// NewGCP constructs an EVM signer backed by the specified GCP KMS key, which
// MUST be a secp256k1-sha256 EC signing key. The SHA256 variant is the only
// secp256k1 signing algorithm supported by GCP KMS, but as it accepts already-
// hashed digests, we simply substitute SHA256 with KECCAK256 as they have the
// same bit length.
//
// Any ClientOptions are propagated to the constructor for the
// KeyManagementClient backing the returned signer.
func NewGCP(ctx context.Context, key string, chainID *big.Int, opts ...option.ClientOption) (*GCP, error) {
	client, err := kms.NewKeyManagementClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("kms.NewKeyManagementClient(…): %v", err)
	}

	req := &kmspb.GetPublicKeyRequest{Name: key}
	pub, err := client.GetPublicKey(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%T.GetPublicKey(ctx, %+v): %v", client, req, err)
	}
	if got, want := pub.Algorithm, kmspb.CryptoKeyVersion_EC_SIGN_SECP256K1_SHA256; got != want {
		return nil, fmt.Errorf("pubkey %q for algorithm %v; MUST be %v", key, got, want)
	}

	block, _ := pem.Decode([]byte(pub.Pem)) // _ = rest []byte, not an unhandled error
	if block == nil {
		return nil, fmt.Errorf("pem.Decode(%T.Pem) returned nil %T", pub, block)
	}
	var info struct {
		Raw       asn1.RawContent
		Algorithm pkix.AlgorithmIdentifier
		PublicKey asn1.BitString
	}
	if rest, err := asn1.Unmarshal(block.Bytes, &info); err != nil || len(rest) > 0 {
		return nil, fmt.Errorf("asn1.Unmarshal(%#x, %T): %d leftover bytes; err=%v", block.Bytes, info, len(rest), err)
	}
	if !info.Algorithm.Algorithm.Equal(oidECDSAPubKey) {
		return nil, errors.New("not an ECDSA public key")
	}

	x, y := elliptic.Unmarshal(crypto.S256(), info.PublicKey.RightAlign())
	pubKey := ecdsa.PublicKey{
		Curve: crypto.S256(),
		X:     x,
		Y:     y,
	}

	return &GCP{
		client: client,
		keyID:  key,
		pubKey: pubKey,
		// The London signer falls back to older signers based on the tx type,
		// so is safe to use as a catch-all.
		signer: types.NewLondonSigner(chainID),
	}, nil
}

// GCP instances are EVM signers backed by Google Cloud's key-management
// service.
type GCP struct {
	client *kms.KeyManagementClient
	keyID  string
	pubKey ecdsa.PublicKey
	signer types.Signer
}

// Close closes the GCP connection.
func (g *GCP) Close() error {
	return g.client.Close()
}

// Address returns the Ethereum address controlled by the signer's private key.
func (g *GCP) Address() common.Address {
	return crypto.PubkeyToAddress(g.pubKey)
}

// SignTx returns tx, signed by the GCP KMS.
func (g *GCP) SignTx(ctx context.Context, tx *types.Transaction) (*types.Transaction, error) {
	req := &kmspb.AsymmetricSignRequest{
		Name: g.keyID,
		// GCP accepts pre-hashed message digests for signing, expecting them to
		// be from one of a limited set of hash functions. We piggyback on the
		// SHA256 option as it has the same number of bits so won't result in an
		// error.
		Digest: &kmspb.Digest{
			Digest: &kmspb.Digest_Sha256{
				Sha256: g.signer.Hash(tx).Bytes(),
			},
		},
	}

	resp, err := g.client.AsymmetricSign(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%T.AsymmetricSign(%+v): %v", g.client, req, err)
	}
	var parsed struct{ R, S *big.Int }
	if rest, err := asn1.Unmarshal(resp.Signature, &parsed); err != nil || len(rest) > 0 {
		return nil, fmt.Errorf("asn1.Unmarshal([sig from GCP KMS], %T): %d bytes left; err=%v", parsed, len(rest), err)
	}

	// EIP-2 limits signatures to one half of the curve
	order := curveOrder()
	halfN := new(big.Int).Rsh(order, 1)
	if parsed.S.Cmp(halfN) == 1 {
		parsed.S.Sub(order, parsed.S)
	}

	sig := make([]byte, 65)
	copy(sig[:32], parsed.R.Bytes())
	copy(sig[32:64], parsed.S.Bytes())

	addr := g.Address()
	// The parity depends on a random value that we don't have access to, so
	// trial-ane-error is the only feasible approach.
	for _, v := range []byte{0, 1} {
		sig[64] = v
		signed, err := tx.WithSignature(g.signer, sig)
		if err != nil {
			return nil, err
		}

		sender, err := types.Sender(g.signer, signed)
		if err != nil {
			return nil, err
		}
		if sender == addr {
			return signed, nil
		}
	}
	return nil, fmt.Errorf("signature doesn't match expected sender address %v", addr)
}
