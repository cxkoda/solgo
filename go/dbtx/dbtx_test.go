package dbtx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/proofxyz/solgo/go/spawner"

	_ "github.com/jackc/pgx/v4/stdlib" // postgres driver
)

func newDB(ctx context.Context, t *testing.T) *sql.DB {
	t.Helper()

	conn := spawner.NewPostgresT(ctx, t, "15", time.Minute)
	db, err := sql.Open("pgx", conn.Dsn)
	if err != nil {
		t.Fatalf("sql.Open(pgx, %T.Dsn = %q) error %v", conn, conn.Dsn, err)
	}
	return db
}

func TestDo(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	const create = `
CREATE TABLE called (
	func_id integer NOT NULL,
	PRIMARY KEY(func_id)
)
`

	insertFunc := func(id int) Func {
		return func(tx *sql.Tx) error {
			const qry = `INSERT INTO called (func_id) VALUES ($1)`
			if _, err := tx.Exec(qry, id); err != nil {
				return fmt.Errorf("%T.Exec(%q, %d) error %v", tx, qry, id, err)
			}
			return nil
		}
	}

	errFail := errors.New("fail")

	tests := []struct {
		name         string
		fns          []Func
		wantInserted []int
		wantErr      error
	}{
		{
			name: "insert multiple",
			fns: []Func{
				insertFunc(0),
				insertFunc(1),
				insertFunc(2),
				insertFunc(3),
			},
			wantInserted: []int{0, 1, 2, 3},
		},
		{
			name: "insert first, fail on second, don't run third",
			fns: []Func{
				insertFunc(0),
				insertFunc(1),
				func(tx *sql.Tx) error {
					if err := insertFunc(2)(tx); err != nil {
						return err
					}
					return errFail
				},
				insertFunc(3),
			},
			wantInserted: []int{0, 1},
			wantErr:      errFail,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			db := newDB(ctx, t)
			if _, err := db.Exec(create); err != nil {
				t.Fatalf("%T.Exec(%q) error %v", db, create, err)
			}

			if err := Do(ctx, db, nil, tt.fns...); !errors.Is(err, tt.wantErr) {
				t.Errorf("Do(…) got err %v; want %v", err, tt.wantErr)
			}

			t.Run("commit vs rollback", func(t *testing.T) {
				const qry = `SELECT func_id FROM called`
				rows, err := db.Query(qry)
				if err != nil {
					t.Fatalf("%T.Query(%q) error %v", db, qry, err)
				}

				var got []int
				for rows.Next() {
					var i int
					if err := rows.Scan(&i); err != nil {
						t.Fatalf("%T.Scan(%T) error %v", rows, &i, err)
					}
					got = append(got, i)
				}
				if err := rows.Err(); err != nil {
					t.Fatalf("%T.Err() = %v", rows, err)
				}

				if diff := cmp.Diff(tt.wantInserted, got); diff != "" {
					t.Errorf("%q results after Do(); diff (-want +got):\n%s", qry, diff)
				}
			})
		})
	}
}

func TestPgTxLock(t *testing.T) {
	ctx := context.Background()
	t.Parallel()

	db := newDB(ctx, t)
	begin := func(t *testing.T) *sql.Tx {
		t.Helper()
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("%T.Begin() error %v", db, err)
		}
		return tx
	}

	const (
		sharedKey = iota
		exclusiveKey
	)

	var txs []*sql.Tx
	for i := 0; i < 3; i++ {
		tx := begin(t)
		txs = append(txs, tx)

		if err := Shared.PgTxLock(ctx, tx, sharedKey); err != nil {
			t.Fatalf("call %d to Shared.PgTxLock(…, %d) error %v", i+1, sharedKey, err)
		}
	}
	for i := 0; i < 3; i++ {
		tx := begin(t)
		txs = append(txs, tx)

		if ok, err := Shared.TryPgTxLock(ctx, tx, sharedKey); err != nil || !ok {
			t.Fatalf("call %d to Shared.PgTxLock(…, %d) got %t, err = %v; want true, nil", i+1, sharedKey, ok, err)
		}
	}

	try := func(t *testing.T, tx *sql.Tx, key int64, want bool) {
		t.Helper()
		if ok, err := Exclusive.TryPgTxLock(ctx, tx, key); err != nil || ok != want {
			t.Errorf("Exclusive.TryPgTxLock(…, %d) got %t, err = %v; want %t, nil", key, ok, err, want)
		}
	}

	t.Run("exclusive lock", func(t *testing.T) {
		t.Logf("After multiple calls to Shared.PgTxLock(…, %d) without ending transactions", sharedKey)
		tx := begin(t)

		t.Run("different key succeeds", func(t *testing.T) {
			if err := Exclusive.PgTxLock(ctx, tx, exclusiveKey); err != nil {
				t.Errorf("Exclusive.PgTxLock(…, %d) error %v", exclusiveKey, err)
			}
		})

		t.Run("same key", func(t *testing.T) {
			t.Run("before committing Txs with shared lock", func(t *testing.T) {
				try(t, tx, sharedKey, false)
			})
			for i, tx := range txs {
				if err := tx.Commit(); err != nil {
					t.Fatalf("%T[%d] holding shared lock; Commit() error %v", tx, i, err)
				}
			}
			t.Run("after committing Txs with shared lock", func(t *testing.T) {
				try(t, tx, sharedKey, true)
			})
		})
	})

	t.Run("already-held exclusive lock", func(t *testing.T) {
		try(t, begin(t), exclusiveKey, false)
	})
}
