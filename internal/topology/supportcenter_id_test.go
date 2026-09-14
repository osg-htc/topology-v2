package topology

import (
	"context"
	"os"
	"testing"

	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// TestImportSupportCenters_ExplicitZeroIDSurvives guards a real bug of the
// same class as the resources_app_created_id_seq / Resource.ID fixes: an
// explicit "ID: 0" (the real "Self Supported" support center in
// support-centers.yaml) was indistinguishable from an absent ID, because
// SupportCenterYAML.ID was a plain int64 rather than a pointer -- so it was
// always silently replaced by the GenID(name) hash fallback. 196 real
// resource groups reference "Self Supported" and would all get a wrong,
// nonzero SupportCenter ID in /rgsummary/xml as a result.
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise, so `go test ./...` stays green without a database.
func TestImportSupportCenters_ExplicitZeroIDSurvives(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	zero := int64(0)
	scs := map[string]SupportCenterYAML{
		"Self Supported": {ID: &zero},
		"No Explicit ID": {}, // ID omitted -- must fall back to GenID(name)
	}
	if err := ImportSupportCenters(ctx, q, scs); err != nil {
		t.Fatalf("ImportSupportCenters: %v", err)
	}

	id, ok := q.SupportCenterIDByName(ctx, "Self Supported")
	if !ok {
		t.Fatalf("SupportCenterIDByName(%q): not found", "Self Supported")
	}
	if id != 0 {
		t.Errorf(`"Self Supported" id = %d, want 0 (explicit ID: 0 must survive, not be replaced by a name hash)`, id)
	}

	wantHash := GenID("No Explicit ID")
	id, ok = q.SupportCenterIDByName(ctx, "No Explicit ID")
	if !ok {
		t.Fatalf("SupportCenterIDByName(%q): not found", "No Explicit ID")
	}
	if id != wantHash {
		t.Errorf("%q id = %d, want %d (GenID fallback for a genuinely absent ID)", "No Explicit ID", id, wantHash)
	}
}
