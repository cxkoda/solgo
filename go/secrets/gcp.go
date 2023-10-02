package secrets

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc/status"
)

// gcpOption wraps a GCP ClientOption in a secrets.Option.
type gcpOption struct {
	Option
	val option.ClientOption
}

// GCPOption returns a Fetch() Option indicating that the secretmanger.Client
// used to fetch the secret must use the ClientOption.
func GCPOption(o option.ClientOption) Option {
	return gcpOption{val: o}
}

// gcp creates a fresh secretmanager.Client, fetches the secret, and returns the
// data. It doesn't verify the payload's checksum because there is no way to
// know which CRC32 table was used.
func gcp(ctx context.Context, name string, opts ...gcpOption) ([]byte, error) {
	clOpts := make([]option.ClientOption, len(opts))
	for i, o := range opts {
		clOpts[i] = o.val
	}

	client, err := secretmanager.NewClient(ctx, clOpts...)
	if err != nil {
		return nil, fmt.Errorf("secretmanager.NewClient(): %v", err)
	}
	defer client.Close()

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}
	resp, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return nil, status.Errorf(status.Code(err), "%T.AccessSecretVersion(…, %+v): %v", client, req, err)
	}

	return resp.Payload.Data, nil
}
