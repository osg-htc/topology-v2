package topology

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// TestImportProjects_MissingIDFallsBackToGenID guards a real bug: v1 always
// resolves a project's ID, falling back to the shared name-hash formula when
// the YAML omits it (project_reader.py: data["ID"] = str(gen_id_from_yaml(...))),
// but v2 never applied this fallback -- a project with no literal ID stored
// an empty ProjectID and rendered with its <ID> element omitted entirely.
// Confirmed against real data: 708 of 1556 real projects have no explicit ID.
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise, so `go test ./...` stays green without a database.
func TestImportProjects_MissingIDFallsBackToGenID(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	dir := t.TempDir()
	// No "ID:" key at all -- the exact real-data shape for 708/1556 projects.
	if err := os.WriteFile(filepath.Join(dir, "Regtest-No-ID-Project.yaml"),
		[]byte("Description: a project with no explicit ID\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	// An explicit ID must still be honored as-is, not overridden.
	if err := os.WriteFile(filepath.Join(dir, "Regtest-Explicit-ID-Project.yaml"),
		[]byte("ID: \"12345\"\nDescription: a project with an explicit ID\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	if err := ImportProjects(ctx, q, dir); err != nil {
		t.Fatalf("ImportProjects: %v", err)
	}

	noID, err := q.GetProjectByName(ctx, "Regtest-No-ID-Project")
	if err != nil {
		t.Fatalf("GetProjectByName(no ID): %v", err)
	}
	if noID.ProjectID == "" {
		t.Fatalf("ProjectID is empty for a project with no literal ID -- the exact bug")
	}
	wantHash := strconv.FormatInt(GenID("Regtest-No-ID-Project"), 10)
	if got := noID.ProjectID; got != wantHash {
		t.Errorf("ProjectID = %q, want %q (GenID fallback)", got, wantHash)
	}

	explicit, err := q.GetProjectByName(ctx, "Regtest-Explicit-ID-Project")
	if err != nil {
		t.Fatalf("GetProjectByName(explicit ID): %v", err)
	}
	if explicit.ProjectID != "12345" {
		t.Errorf("ProjectID = %q, want %q (explicit ID must be honored, not overridden)", explicit.ProjectID, "12345")
	}
}
