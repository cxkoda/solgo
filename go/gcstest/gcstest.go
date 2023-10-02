// Package gcstest assists in testing code that relies on Google Cloud Storage.
package gcstest

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"google.golang.org/api/option"
)

// SignedURLClientOption constructs JSON credentials for a Service Account with
// the provided email address and a deterministic private key, the seed for
// which MAY be nil.
//
// If the ClientOption is passed to storage.NewClient(), the credentials will be
// used by all attempts to sign URLs with BucketHandles obtained via the Client.
// As a result, no network calls should be made and the signing is entirely
// self-contained. The returned URLs will be syntactically but not
// semantically valid; i.e. they are well-formed but won't provide access to
// anything.
func SignedURLClientOption(tb testing.TB, clientEmail string, privateKeySeed []byte) option.ClientOption {
	tb.Helper()

	s := crypto.NewKeccakState()
	if _, err := s.Write(privateKeySeed); err != nil {
		tb.Fatalf("%T.Write(%s) error %v", s, privateKeySeed, err)
	}

	const bits = 2048
	key, err := rsa.GenerateKey(s, bits)
	if err != nil {
		tb.Fatalf("rsa.GenerateKey(%T[%q], %d) error %v", s, privateKeySeed, bits, err)
	}
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	var keyPEM bytes.Buffer
	if err := pem.Encode(&keyPEM, block); err != nil {
		tb.Fatalf("pem.Encode(…) error %v", err)
	}

	creds := struct {
		Type  string `json:"type"`
		Email string `json:"client_email"`
		PK    string `json:"private_key"`
	}{
		Type:  "service_account",
		Email: clientEmail,
		PK:    keyPEM.String(),
	}

	buf, err := json.Marshal(creds)
	if err != nil {
		tb.Fatalf("json.Marshal(%T) error %v", creds, err)
	}
	return option.WithCredentialsJSON(buf)
}
