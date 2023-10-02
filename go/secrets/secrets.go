// Package secrets provides mechanisms for referring to and accessing sensitive
// information, NOT including cryptographic keys. These can be, for example,
// passwords, API tokens, or OAuth client secrets.
package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A Sources defines the source of the secret.
type Source string

const (
	// The Raw Source carries a raw, unprotected "secret"; it MUST NOT be used
	// for sensitive data, and is exposed to allow raw data when a Secret string
	// is expected.
	Raw Source = "not-secret"
	// The GCP Source fetches secrets from the GCP Secrets Manager.
	GCP Source = "gcp"
	// The Environment Source fetches secrets from an environment variable.
	Environment Source = "env"
)

// A Secret identifies a secret but doesn't carry the value itself.
type Secret struct {
	Source Source
	ID     string
}

// String returns <s.Source>://<s.ID>; e.g. env://MY_VAR to describe $MY_VAR.
// If the secret is nil, this will return an invalid secret string.
func (s *Secret) String() string {
	if s == nil {
		return fmt.Sprintf("invalid (nil) %T", s)
	}
	return fmt.Sprintf("%s://%s", s.Source, s.ID)
}

// Set is the inverse of s.String(). Together, these mean that *Secret
// implements flag.Value, for use with flag.Var().
func (s *Secret) Set(raw string) error {
	parts := strings.SplitN(raw, "://", 2)
	if len(parts) != 2 {
		return status.Errorf(codes.InvalidArgument, "invalid %T string %q", s, raw)
	}

	s.Source = Source(parts[0])
	switch s.Source {
	case Raw, GCP, Environment:
	default:
		return status.Errorf(codes.InvalidArgument, "invalid %T %q from %q", s.Source, s.Source, raw)
	}
	s.ID = parts[1]

	return nil
}

// Type returns the fully qualified type of s.
// Required for use with pflag to implement the pflag.Value interface.
func (s *Secret) Type() string {
	return fmt.Sprintf("%T", s)
}

// An Option configures behaviour of Secret.Fetch().
type Option interface {
	isOption() // can't be implemented outside this package
}

// Fetch fetches and returns the Secret's payload. It ignores all Options that
// aren't relevant to s.Source; for example, passing a GCPOption with an
// environment variable is allowed.
func (s *Secret) Fetch(ctx context.Context, opts ...Option) ([]byte, error) {
	switch s.Source {
	case GCP:
		return gcp(ctx, s.ID, filterOptions[gcpOption](opts)...)

	case Raw:
		return []byte(s.ID), nil

	case Environment:
		val, ok := os.LookupEnv(s.ID)
		if !ok {
			return nil, status.Errorf(codes.NotFound, "environment variable %q not set", s.ID)
		}
		return []byte(val), nil

	default:
		return nil, status.Errorf(codes.Unimplemented, "unsupported secret source %q", s.Source)
	}
}

// filterOptions is a generic function to filter a slice of Options into those
// of a specific concrete type.
func filterOptions[O Option](opts []Option) []O {
	var res []O
	for _, o := range opts {
		if cast, ok := o.(O); ok {
			res = append(res, cast)
		}
	}
	return res
}
