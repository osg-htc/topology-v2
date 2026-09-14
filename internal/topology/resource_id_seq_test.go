package topology

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// TestImport_ResyncsAppCreatedResourceIDSeq guards a real bug: migration 011
// seeds resources_app_created_id_seq exactly once, at migration time, against
// whatever's in the (almost always empty) database then -- on a freshly
// migrated instance that seed is 1, and nothing ever re-synced it against
// real data afterwards. Every app-created resource then drew its id from a
// sequence stuck near 1, a live collision risk against any legacy resource
// with a small hand-assigned id -- this repo's own fixture data has one
// (UChicago_OSGConnect_ap20, explicit ID 1437). Import must leave the
// sequence past the highest explicit id it just loaded.
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise, so `go test ./...` stays green without a database.
func TestImport_ResyncsAppCreatedResourceIDSeq(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	tree, err := ReadTree(filepath.Join("testdata", "topology"))
	if err != nil {
		t.Fatalf("ReadTree: %v", err)
	}
	if err := Import(ctx, q, tree); err != nil {
		t.Fatalf("Import: %v", err)
	}

	const fixtureMaxExplicitID = 1437 // UChicago_OSGConnect_ap20's ID in testdata
	next, err := q.NextAppCreatedResourceID(ctx)
	if err != nil {
		t.Fatalf("NextAppCreatedResourceID: %v", err)
	}
	if next <= fixtureMaxExplicitID {
		t.Fatalf("NextAppCreatedResourceID = %d, want > %d (the fixture's own highest explicit resource id) -- "+
			"an app-created resource would collide with imported legacy data", next, fixtureMaxExplicitID)
	}
}
