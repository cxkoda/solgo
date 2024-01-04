// Package spawner implements the Spawner service to launch arbitrary binaries
// for use in tests.
package spawner

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/uuid"
	"github.com/ory/dockertest/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/cxkoda/solgo/go/spawner/proto"
)

// New returns a newly constructed SpawnerServiceServer. Set poolMaxWait to 0 to
// use the default dockertest.Pool value (1 minute at time of writing).
func New(poolMaxWait time.Duration) (pb.SpawnerServiceServer, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, fmt.Errorf("dockertest.NewPool(%q) error %v", "", err)
	}
	pool.MaxWait = poolMaxWait

	return &spawner{
		pool:      pool,
		resources: make(map[uuid.UUID]*dockertest.Resource),
	}, nil
}

// NewT is a convenience function for using New() in Go tests. All errors are
// reported on tb.Fatal().
func NewT(tb testing.TB, poolMaxWait time.Duration) pb.SpawnerServiceServer {
	tb.Helper()

	s, err := New(poolMaxWait)
	if err != nil {
		tb.Fatalf("spawner.New(%s) error %v", poolMaxWait, err)
	}
	return s
}

// spawner implements the SpawnerService server interface.
type spawner struct {
	pool *dockertest.Pool

	resourceMu sync.Mutex
	resources  map[uuid.UUID]*dockertest.Resource
}

type launcher func(context.Context, *pb.SpawnRequest) (*dockertest.Resource, *pb.SpawnResponse, error)

// Spawn spawns the requested process type.
func (s *spawner) Spawn(ctx context.Context, req *pb.SpawnRequest) (_ *pb.SpawnResponse, retErr error) {
	var launch launcher
	switch req.Process.(type) {
	case *pb.SpawnRequest_Postgres:
		launch = s.postgres

	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid %T.process: %T", req, req.Process)
	}

	resource, resp, err := launch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("spawning %T: %v", req.Process, err)
	}
	defer func() {
		if retErr != nil {
			if err := s.kill(resource); err != nil {
				retErr = fmt.Errorf("%w [failed to kill already-spawned process: %v]", retErr, err)
			}
		}
	}()

	id, err := uuid.NewRandom()
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "uuid.NewRandom(): %v", err)
	}
	resp.ToKill = &pb.KillRequest{
		ProcId: id[:],
	}

	s.resourceMu.Lock()
	defer s.resourceMu.Unlock()
	s.resources[id] = resource

	return resp, nil
}

// Kill kills the requested "process". The request MUST be propagated from the
// value included in a SpawnResponse.
func (s *spawner) Kill(ctx context.Context, req *pb.KillRequest) (*empty.Empty, error) {
	var id uuid.UUID
	if err := id.UnmarshalBinary(req.ProcId); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "uuid.ParseBytes(%#x): %v", req.ProcId, err)
	}

	s.resourceMu.Lock()
	defer s.resourceMu.Unlock()
	res, ok := s.resources[id]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "process ID %#x not found", id)
	}

	return &empty.Empty{}, s.kill(res)
}

func (s *spawner) kill(res *dockertest.Resource) error {
	if err := s.pool.Purge(res); err != nil {
		return fmt.Errorf("%T.Purge(): %v", s.pool, res)
	}
	return nil
}
