package spawner

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/proofxyz/solgo/go/grpctest"
	"github.com/proofxyz/solgo/go/protovalid"
	pb "github.com/proofxyz/solgo/go/spawner/proto"
)

func TestPostgres(t *testing.T) {
	ctx := context.Background()

	svc := NewT(t, 5*time.Second)
	validate := grpc.UnaryInterceptor(protovalid.UnaryServerInterceptor)
	conn := grpctest.NewClientConnTB(t, pb.RegisterSpawnerServiceServer, svc, validate)
	client := pb.NewSpawnerServiceClient(conn)

	// fullGRPCSpawn has the same signature as NewPostgresT, which doesn't use
	// the full gRPC stack and instead calls the SpawnerServiceServer directly.
	fullGRPCSpawn := func(ctx context.Context, tb testing.TB, ver string, ttl time.Duration) *pb.PostgresResponse {
		t.Helper()

		req := &pb.SpawnRequest{
			Process: &pb.SpawnRequest_Postgres{
				Postgres: &pb.PostgresRequest{
					DockerTag: ver,
					Ttl:       durationpb.New(ttl),
				},
			},
		}

		resp, err := client.Spawn(ctx, req)
		if err != nil {
			tb.Fatalf("%T.Spawn(ctx, %+v) error %v", client, req, err)
		}

		tb.Cleanup(func() {
			if _, err := client.Kill(ctx, resp.ToKill); err != nil {
				t.Errorf("%T.Kill(…) error %v", client, err)
			}
		})

		return resp.GetPostgres()
	}

	tests := []struct {
		name    string
		version string
		spawn   func(context.Context, testing.TB, string, time.Duration) *pb.PostgresResponse
	}{
		{
			name:    "in-process without gRPC stack",
			version: "14.5",
			spawn:   NewPostgresT,
		},
		{
			name:    "in-process without gRPC stack",
			version: "15.0",
			spawn:   NewPostgresT,
		},
		{
			name:    "full gRPC stack",
			version: "14.5",
			spawn:   fullGRPCSpawn,
		},
		{
			name:    "full gRPC stack",
			version: "15.0",
			spawn:   fullGRPCSpawn,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("%s; version = %q", tt.name, tt.version), func(t *testing.T) {
			t.Parallel()

			resp := tt.spawn(ctx, t, tt.version, 3*time.Minute)

			dsn := resp.Dsn
			db, err := sql.Open("pgx", dsn)
			if err != nil {
				t.Fatalf("sql.Open(pgx, %q) error %v", dsn, err)
			}
			t.Cleanup(func() { db.Close() })

			row := db.QueryRow(`SELECT version()`)
			var got string
			if err := row.Scan(&got); err != nil {
				t.Fatalf("%T.Scan(%T) error %v", row, &got, err)
			}
			if want := tt.version; !strings.Contains(got, want) {
				t.Errorf("SELECT version() got %q; want containing %q", got, want)
			}
		})
	}
}
