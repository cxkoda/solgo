// Package dbtx provides convenience functions for database transactions.
package dbtx

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/cxkoda/solgo/go/memconv"
)

// A Func is a function that performs actions within the scope of a single
// database transaction. It MUST NOT call Commit() nor Rollback().
type Func func(*sql.Tx) error

// A Beginner can begin transactions. Both sql.Conn and sql.DB are Beginners.
type Beginner interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

// A Transactor manages transactions from beginning to end, passing off all
// application-specific logic to a set of Funcs. Unlike use of the top-level
// Do() function, wrapping a database connection in a Transactor forces all
// database access to be in transactions.
type Transactor struct {
	Beginner
}

// Do begins one new transaction on the Beginner per Func fn() in fns. The
// transaction is commited if fn() returns a nil error, otherwise it is rolled
// back, the error propagated, and all further Funcs ignored.
//
// See PgTxLock() re unlocking as rationale for accepting multiple functions.
// Note, however, that Do() is agnostic to the underlying database type.
func (t Transactor) Do(ctx context.Context, opts *sql.TxOptions, fns ...Func) error {
	for _, fn := range fns {
		tx, err := t.BeginTx(ctx, opts)
		if err != nil {
			return fmt.Errorf("%T.BeginTx(%+v): %v", t.Beginner, opts, err)
		}
		if err := fn(tx); err != nil {
			return multierror.Append(err, tx.Rollback()) // nil error on Rollback is ignored
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Do is a convenience wrapper for Transactor{b}.Do(). Code that repeatedly
// calls Do() MAY wish to use a Transactor to avoid having to constantly pass
// the Beginner, and to force users to wrap all access in transactions, but both
// will always be identical.
func Do(ctx context.Context, b Beginner, opts *sql.TxOptions, fns ...Func) error {
	return Transactor{b}.Do(ctx, opts, fns...)
}

// Exclusivity defines a type of exclusivity for locks. How each type is treated
// is dependent on the function which it is passed.
type Exclusivity int

// Exclusivity options.
const (
	UndefinedExclusivity Exclusivity = iota
	Exclusive
	Shared
)

// PgTxLock obtains a PostgreSQL transaction-level lock; i.e. either a
// [pg_advisory_xact_lock] or the _shared equivalent if e is Exclusive or
// Shared, respectively.
//
// Such locks can't be explicitly unlocked and are automatically released at the
// end of the transaction.  Transactions using PgTxLock() SHOULD therefore be
// short-lived and limited to the minimal set of isolated logic—this is why Do()
// accepts multiple functions.
//
// [pg_advisory_xact_lock] https://www.postgresql.org/docs/15/functions-admin.html#FUNCTIONS-ADVISORY-LOCKS
func (e Exclusivity) PgTxLock(ctx context.Context, tx *sql.Tx, key int64) error {
	var qry string
	switch e {
	case Exclusive:
		qry = "SELECT pg_advisory_xact_lock($1)"
	case Shared:
		qry = "SELECT pg_advisory_xact_lock_shared($1)"
	default:
		return fmt.Errorf("unsupported %T: %d", e, e)
	}

	if _, err := tx.ExecContext(ctx, qry, key); err != nil {
		return fmt.Errorf("obtain transaction-level advisory lock %d: %v", key, err)
	}
	return nil
}

// TryPgTxLock tries to obtain a lock, similarly to PgTxLock(). Failure to
// obtain the lock does not constitute an error, and the returned boolean MUST
// be checked.
func (e Exclusivity) TryPgTxLock(ctx context.Context, tx *sql.Tx, key int64) (bool, error) {
	var qry string
	switch e {
	case Exclusive:
		qry = "SELECT pg_try_advisory_xact_lock($1)"
	case Shared:
		qry = "SELECT pg_try_advisory_xact_lock_shared($1)"
	default:
		return false, fmt.Errorf("unsupported %T: %d", e, e)
	}

	r := tx.QueryRowContext(ctx, qry, key)
	var ok bool
	if err := r.Scan(&ok); err != nil {
		return false, fmt.Errorf("trying to obtain transaction-level advisory lock %d: %v", key, err)
	}
	return ok, nil
}

// PgLockKey returns an int64 for use as a key in obtaining a PostgreSQL
// advisory lock. The returned value is derived from interpretting the first 8
// bytes of the sha256 sum of key as an int64.
func PgLockKey(key string) int64 {
	h := sha256.Sum256([]byte(key))
	var b [8]byte
	copy(b[:], h[:])
	return memconv.Cast[[8]byte, int64](&b)
}
