package xmlapi_test

import (
	"context"
	"os"
	"testing"

	"github.com/bbockelm/topology-v2/internal/testsupport"
	"github.com/bbockelm/topology-v2/internal/xmlapi"
)

// TestBuildVOSummary_ActiveIndependentOfDisable guards a bug of the same
// class as the already-fixed Resource.Active/Disable ones: VO.Active was
// derived as `!Disable` instead of being read from its own independent YAML
// key (v1 defaults Active=true, Disable=false, both independently, before
// overlaying the VO's real data -- see vos_data.py's _expand_vo). Confirmed
// against real data: 10 real VOs have Active=false, Disable=false set
// independently, and rendered Active=true before this fix.
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise, so `go test ./...` stays green without a database.
func TestBuildVOSummary_ActiveIndependentOfDisable(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	// Active omitted -- v1 default is true.
	if err := q.UpsertVO(ctx, "regtest-vo-active-omitted", 900001, false,
		[]byte("LongName: Regtest VO\n")); err != nil {
		t.Fatalf("UpsertVO (active omitted): %v", err)
	}
	// The exact live-bug pattern: Active=false, Disable=false, independently.
	if err := q.UpsertVO(ctx, "regtest-vo-active-false-disable-false", 900002, false,
		[]byte("LongName: Regtest VO\nActive: false\n")); err != nil {
		t.Fatalf("UpsertVO (active false, disable false): %v", err)
	}
	// Active=true, Disable=true -- proves Active is never forced false just
	// because Disable is true.
	if err := q.UpsertVO(ctx, "regtest-vo-active-true-disable-true", 900003, true,
		[]byte("LongName: Regtest VO\nActive: true\n")); err != nil {
		t.Fatalf("UpsertVO (active true, disable true): %v", err)
	}

	summary, err := xmlapi.BuildVOSummary(ctx, q, false)
	if err != nil {
		t.Fatalf("BuildVOSummary: %v", err)
	}
	byName := map[string]xmlapi.VOXML{}
	for _, v := range summary.VOs {
		byName[v.Name] = v
	}

	cases := []struct {
		name        string
		wantActive  bool
		wantDisable bool
	}{
		{"regtest-vo-active-omitted", true, false},
		{"regtest-vo-active-false-disable-false", false, false},
		{"regtest-vo-active-true-disable-true", true, true},
	}
	for _, c := range cases {
		v, ok := byName[c.name]
		if !ok {
			t.Fatalf("VO %q not found in rendered summary", c.name)
		}
		if v.Active != c.wantActive {
			t.Errorf("%s: Active = %v, want %v", c.name, v.Active, c.wantActive)
		}
		if v.Disable != c.wantDisable {
			t.Errorf("%s: Disable = %v, want %v", c.name, v.Disable, c.wantDisable)
		}
	}
}
