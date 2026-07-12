package topology

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// TestDBRoundTrip proves the full backup/restore round-trip through Postgres:
// read the fixture tree -> Import into the DB -> Export from the DB -> write the
// tree back out -> assert structural equivalence (modulo whitespace).
//
// It requires a Postgres reachable at TOPOLOGY_TEST_DATABASE_URL and is skipped
// otherwise, so `go test ./...` stays green without a database.
func TestDBRoundTrip(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run the DB round-trip test")
	}

	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	// Optionally round-trip a real topology tree (e.g. a clone of the OSG repo)
	// to catch modeling gaps in production data.
	src := filepath.Join("testdata", "topology")
	if real := os.Getenv("TOPOLOGY_TEST_REAL_TREE"); real != "" {
		src = real
	}
	tree, err := ReadTree(src)
	if err != nil {
		t.Fatalf("ReadTree: %v", err)
	}
	if err := Import(ctx, q, tree); err != nil {
		t.Fatalf("Import: %v", err)
	}
	exported, err := Export(ctx, q)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	dst := t.TempDir()
	if err := WriteTree(dst, exported); err != nil {
		t.Fatalf("WriteTree: %v", err)
	}
	assertTreesEquivalent(t, src, dst)
}
