package spawner

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"

	pb "github.com/proofxyz/solgo/go/spawner/proto"

	_ "github.com/jackc/pgx/v4/stdlib" // postgres driver
)

func (s *spawner) postgres(ctx context.Context, sReq *pb.SpawnRequest) (*dockertest.Resource, *pb.SpawnResponse, error) {
	req := sReq.GetPostgres()

	const (
		user           = "user"
		password       = "password"
		dataDirPattern = "postgresql-data-*"
	)
	if req.DbName == "" {
		req.DbName = "dbname"
	}
	dataDir, err := os.MkdirTemp("", dataDirPattern)
	if err != nil {
		return nil, nil, fmt.Errorf("os.MkdirTemp(%q, %q): %v", "", dataDirPattern, err)
	}

	opts := &dockertest.RunOptions{
		Repository: "postgres",
		Tag:        req.DockerTag,
		Env: []string{
			fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
			fmt.Sprintf("POSTGRES_USER=%s", user),
			fmt.Sprintf("POSTGRES_DB=%s", req.DbName),
			fmt.Sprintf("PGDATA=%s", dataDir),
			"listen_addresses = '*'",
		},
	}
	res, err := s.pool.RunWithOptions(
		opts,
		func(hostCfg *docker.HostConfig) {
			hostCfg.AutoRemove = true
			hostCfg.RestartPolicy = docker.NeverRestart()
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("%T.RunWithOptions(%+v) error %v", s.pool, opts, err)
	}
	ttl := req.Ttl.AsDuration()
	res.Expire(uint(ttl / time.Second))

	var (
		db *sql.DB
		// pool.Retry doesn't propagate errors so we need to do it ourselves
		lastConnErr error
	)
	hostPort := res.GetHostPort("5432/tcp")
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", user, password, hostPort, url.PathEscape(req.DbName))
	tryConnect := func() (retErr error) {
		defer func() { lastConnErr = retErr }()

		db, err = sql.Open("pgx", dsn)
		if err != nil {
			return err
		}
		return db.PingContext(ctx)
	}

	if err := s.pool.Retry(tryConnect); err != nil {
		return nil, nil, fmt.Errorf("%T.Retry(sql.Open(pgx, %q)) error %v; last retry error: %v", s.pool, dsn, err, lastConnErr)
	}
	if err := db.Close(); err != nil {
		return nil, nil, fmt.Errorf("%T.Close(): %v", db, err)
	}

	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		return nil, nil, fmt.Errorf("net.SplitHostPort(%q): %v", hostPort, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing port from %q: strconv.Atoi(%q): %v", hostPort, portStr, err)
	}

	return res, &pb.SpawnResponse{
		Process: &pb.SpawnResponse_Postgres{Postgres: &pb.PostgresResponse{
			Dsn:         dsn,
			User:        user,
			Password:    password,
			HostAndPort: hostPort,
			Host:        host,
			Port:        int64(port),
			DbName:      req.DbName,
		}},
	}, nil
}
