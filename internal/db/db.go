// Package db owns the Postgres connection pool and the hand-written SQL query
// layer (no ORM), following the SWAMP/FabAID pattern: a Queries struct wrapping
// a *pgxpool.Pool with one method per operation.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pgx connection pool and verifies connectivity.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database url: %w", err)
	}
	cfg.MaxConns = 10

	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return pool, nil
}

// DBTX is the subset of pgx used by the query methods, satisfied by both
// *pgxpool.Pool and pgx.Tx. Binding Queries to this interface lets a bundle of
// operations run inside a single transaction (see WithTx).
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Queries wraps the connection (pool or transaction) with hand-written SQL
// methods. Domain-specific methods live in sibling files (queries_*.go).
type Queries struct {
	pool DBTX          // used by every query method; a pool or an active tx
	raw  *pgxpool.Pool // concrete pool, for Pool() and starting transactions
}

// New returns a Queries bound to the given pool.
func New(pool *pgxpool.Pool) *Queries {
	return &Queries{pool: pool, raw: pool}
}

// Pool exposes the underlying pool for transactions and advanced use.
func (q *Queries) Pool() *pgxpool.Pool {
	return q.raw
}

// WithTx runs fn with a Queries bound to a fresh transaction, committing on
// success and rolling back on any error. Used to apply a bundle of dependent
// operations atomically.
func (q *Queries) WithTx(ctx context.Context, fn func(tx *Queries) error) error {
	tx, err := q.raw.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful commit
	if err := fn(&Queries{pool: tx, raw: q.raw}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
