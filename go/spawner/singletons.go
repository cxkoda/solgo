package spawner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	pb "github.com/cxkoda/solgo/go/spawner/proto"
)

// Convenience functions for spawning one-off "processes" without an independent
// SpawnerService.

// NewPostgres spawns a new PostgreSQL instance. The returned cleanup function
// MUST be called to kill the instance.
func NewPostgres(ctx context.Context, dockerTag string, ttl time.Duration) (*pb.PostgresResponse, func(context.Context) error, error) {
	s, err := New(30 * time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("spawner.New(): %v", err)
	}

	req := &pb.SpawnRequest{
		Process: &pb.SpawnRequest_Postgres{
			Postgres: &pb.PostgresRequest{
				DockerTag: dockerTag,
				Ttl:       durationpb.New(ttl),
			},
		},
	}
	resp, err := s.Spawn(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("%T.Spawn(%T %+v) error %v", s, req, req, err)
	}
	cleanup := func(ctx context.Context) error {
		_, err := s.Kill(ctx, resp.ToKill)
		return err
	}
	return resp.GetPostgres(), cleanup, nil
}

// NewPostgresT is equivalent to NewPostGres() except that failures to spawn are
// reported on tb.Fatal(), and Kill() is called in tb.Cleanup().
func NewPostgresT(ctx context.Context, tb testing.TB, dockerTag string, ttl time.Duration) *pb.PostgresResponse {
	tb.Helper()

	resp, cleanup, err := NewPostgres(ctx, dockerTag, ttl)
	if err != nil {
		tb.Fatal(err)
	}

	tb.Cleanup(func() {
		if err := cleanup(ctx); err != nil {
			tb.Errorf("cleanup() as returned by NewPostgres(); error %v", err)
		}
	})

	return resp
}
