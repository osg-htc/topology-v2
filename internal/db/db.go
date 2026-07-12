// Package db owns the Postgres connection pool and the hand-written SQL query
// layer (no ORM), following the SWAMP/FabAID pattern: a Queries struct wrapping
// a *pgxpool.Pool with one method per operation.
package db

import (
	"context"
	"fmt"
	"time"

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

// Queries wraps the connection pool with hand-written SQL methods.
// Domain-specific methods live in sibling files (queries_*.go).
type Queries struct {
	pool *pgxpool.Pool
}

// New returns a Queries bound to the given pool.
func New(pool *pgxpool.Pool) *Queries {
	return &Queries{pool: pool}
}

// Pool exposes the underlying pool for transactions and advanced use.
func (q *Queries) Pool() *pgxpool.Pool {
	return q.pool
}
