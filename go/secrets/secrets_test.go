package secrets

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/h-fam/errdiff"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cxkoda/solgo/go/grpctest"
)

func TestSecretEndToEnd(t *testing.T) {
	ctx := context.Background()

	const (
		unsetEnvVar = "secrets-test-unset-env-var"
		setEnvVar   = "secrets-test-set-env-var"
		envVarVal   = "TOP SECRET"
	)
	if err := os.Unsetenv(unsetEnvVar); err != nil {
		t.Fatalf("os.Unsetenv(%q) error %v", unsetEnvVar, err)
	}
	if err := os.Setenv(setEnvVar, envVarVal); err != nil {
		t.Fatalf("os.Setenv(%q, %q) error %v", setEnvVar, envVarVal, err)
	}

	tests := []struct {
		name               string
		flagValue          string
		flagErrDiffAgainst interface{}
		fetchOpts          []Option
		want               []byte
		errDiffAgainst     interface{}
	}{
		{
			name:               "invalid string",
			flagValue:          "Flaggy McFlagface",
			flagErrDiffAgainst: "Flaggy McFlagface",
		},
		{
			name:               "invalid type",
			flagValue:          Source("foo").flagValue("hello"),
			flagErrDiffAgainst: "foo",
		},
		{
			name:      "raw value: hello",
			flagValue: Raw.flagValue("hello"),
			want:      []byte("hello"),
		},
		{
			name:      "raw value: world",
			flagValue: Raw.flagValue("world"),
			want:      []byte("world"),
		},
		{
			name:           "unset env variable",
			flagValue:      Environment.flagValue(unsetEnvVar),
			errDiffAgainst: codes.NotFound,
		},
		{
			name:      "set env variable",
			flagValue: Environment.flagValue(setEnvVar),
			want:      []byte(envVarVal),
		},
		{
			name:      "GCP secret",
			flagValue: GCP.flagValue(gcpSecretName),
			fetchOpts: []Option{gcpStubOption(t)},
			want:      []byte(gcpSecretValue),
		},
		{
			name:           "non-existent GCP secret",
			flagValue:      GCP.flagValue("non-existent"),
			fetchOpts:      []Option{gcpStubOption(t)},
			errDiffAgainst: codes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := new(Secret)

			fset := flag.NewFlagSet(tt.name, flag.ContinueOnError)
			const flagName = "secret"
			fset.Var(s, flagName, "")

			if diff := errdiff.Check(fset.Set(flagName, tt.flagValue), tt.flagErrDiffAgainst); diff != "" {
				t.Errorf("%s", diff)
			}
			if tt.flagErrDiffAgainst != nil {
				return
			}

			got, err := s.Fetch(ctx, tt.fetchOpts...)
			if diff := errdiff.Check(err, tt.errDiffAgainst); diff != "" {
				t.Errorf("%s", diff)
			}
			if tt.errDiffAgainst != nil {
				return
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
}

func TestSecretUninitialised(t *testing.T) {
	var secret *Secret
	want := "invalid (nil) *secrets.Secret"
	if got := secret.String(); got != want {
		t.Fatalf("String(): got %q; want %q", got, want)
	}
}

// flagValue returns the string to pass as a flag to request a secret from the
// Source with a specified id.
func (s Source) flagValue(id string) string {
	return fmt.Sprintf("%s://%s", s, id)
}

// gcpStubOption returns a Fetch() Option that configures all gcp:// secrets to
// be fetched from a new gcpSecretsStub{}.
func gcpStubOption(t *testing.T) Option {
	t.Helper()
	var stub secretmanagerpb.SecretManagerServiceServer = &gcpSecretsStub{}
	conn := grpctest.NewClientConnTB(t, secretmanagerpb.RegisterSecretManagerServiceServer, stub)
	return GCPOption(option.WithGRPCConn(conn))
}

// gcpSecretsStub implements the AccessSecretVersion() gRPC method of the GCP
// SecretManagerService.
type gcpSecretsStub struct {
	secretmanagerpb.UnimplementedSecretManagerServiceServer
}

const (
	gcpSecretName  = "projects/the-project/secrets/the-secret/versions/latest"
	gcpSecretValue = "DID-YOU-KNOW-THAT-KEVIN-…"
)

func (s *gcpSecretsStub) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	if req.Name != gcpSecretName {
		return nil, status.Errorf(codes.NotFound, "secret %q not found", req.Name)
	}

	return &secretmanagerpb.AccessSecretVersionResponse{
		Name: req.Name,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(gcpSecretValue),
		},
	}, nil
}
