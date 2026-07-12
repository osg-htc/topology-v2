// Package testsupport provides shared helpers for DB-backed integration tests.
//
// Integration tests are gated on TOPOLOGY_TEST_DATABASE_URL and, because
// `go test ./...` runs packages concurrently, each test must be isolated from
// the others. SetupSchema gives every caller its own uniquely-named Postgres
// schema (via search_path) with migrations applied, dropped on cleanup — so
// concurrent DB tests never clobber one another.
package testsupport

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bbockelm/topology-v2/internal/db"
)

// SetupSchema creates an isolated schema, runs migrations into it, and returns a
// pool + Queries bound to it. The schema is dropped when the test finishes.
// Returns ("", nil, false) if TOPOLOGY_TEST_DATABASE_URL is unset (caller skips).
func SetupSchema(t *testing.T, dbURL string) (*pgxpool.Pool, *db.Queries) {
	t.Helper()

	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	ctx := context.Background()

	// Create the schema on a base connection (no search_path needed).
	base, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect base: %v", err)
	}
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		base.Close()
		t.Fatalf("create schema: %v", err)
	}
	base.Close()

	schemaURL := withSearchPath(dbURL, schema)
	if err := db.RunMigrations(schemaURL); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	pool, err := db.Connect(ctx, schemaURL)
	if err != nil {
		t.Fatalf("connect schema: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		drop, err := pgxpool.New(ctx, dbURL)
		if err == nil {
			_, _ = drop.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
			drop.Close()
		}
	})

	return pool, db.New(pool)
}

// withSearchPath appends a search_path runtime parameter to a Postgres URL.
// pgx passes unrecognized query keys through as connection runtime parameters,
// so this scopes both the pool and goose (via pgx stdlib) to the schema.
func withSearchPath(dbURL, schema string) string {
	sep := "?"
	if strings.Contains(dbURL, "?") {
		sep = "&"
	}
	return dbURL + sep + "search_path=" + schema
}
