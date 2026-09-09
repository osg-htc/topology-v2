package xmlapi_test

import (
	"context"
	"os"
	"testing"

	"github.com/bbockelm/topology-v2/internal/testsupport"
	"github.com/bbockelm/topology-v2/internal/topology"
	"github.com/bbockelm/topology-v2/internal/xmlapi"
)

func boolPtr(b bool) *bool { return &b }

// TestBuildResourceSummary_ActiveDefaultAndIndependentDisable guards the two
// highest-blast-radius data-model bugs found in the v1<->v2 conformance
// audit (docs/SPEC_CONFORMANCE_AUDIT.md §2/§6):
//
//  1. Resource.Active's default was inverted: an omitted Active rendered as
//     false instead of v1's documented default of true (18 real resources
//     affected).
//  2. Resource.Disable was fabricated as `!Active` instead of being its own
//     independent field, so any resource with Active=false, Disable=false
//     (independently, per v1's real source data) wrongly rendered
//     Disable=true (338 real resources affected).
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise, so `go test ./...` stays green without a database.
func TestBuildResourceSummary_ActiveDefaultAndIndependentDisable(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	groupID := int64(900000200)
	tree := &topology.Topology{
		Facilities: map[string]*topology.Facility{
			"regtest-active-disable-facility": {Name: "regtest-active-disable-facility"},
		},
		Sites: map[string]*topology.Site{
			"regtest-active-disable-site": {Name: "regtest-active-disable-site"},
		},
		ResourceGroups: map[string]*topology.ResourceGroup{
			"regtest-active-disable-rg": {
				Name:    "regtest-active-disable-rg",
				GroupID: &groupID,
				Resources: map[string]*topology.Resource{
					// Active/Disable both omitted -- the v1 spec default is
					// Active=true, Disable=false.
					"res-both-omitted": {FQDN: "both-omitted.example.org"},
					// The exact 338-resource live bug pattern: Active=false
					// and Disable=false are independently set. Before the
					// fix, Disable was fabricated as !Active (true) here.
					"res-active-false-disable-false": {
						FQDN:   "active-false-disable-false.example.org",
						Active: boolPtr(false), Disable: boolPtr(false),
					},
					// Active=false, Disable=true -- both explicitly set and
					// happen to agree with the old fabrication, to prove
					// this case still renders correctly too.
					"res-active-false-disable-true": {
						FQDN:   "active-false-disable-true.example.org",
						Active: boolPtr(false), Disable: boolPtr(true),
					},
					// Active=true, Disable=true -- proves Disable is never
					// silently forced false just because Active is true.
					"res-active-true-disable-true": {
						FQDN:   "active-true-disable-true.example.org",
						Active: boolPtr(true), Disable: boolPtr(true),
					},
				},
			},
		},
		Downtimes:    map[string][]*topology.Downtime{},
		SiteFacility: map[string]string{"regtest-active-disable-site": "regtest-active-disable-facility"},
		RGSite:       map[string]string{"regtest-active-disable-rg": "regtest-active-disable-site"},
	}
	if err := topology.Import(ctx, q, tree); err != nil {
		t.Fatalf("Import: %v", err)
	}

	summary, err := xmlapi.BuildResourceSummary(ctx, q, nil, xmlapi.Filters{}, false)
	if err != nil {
		t.Fatalf("BuildResourceSummary: %v", err)
	}

	var rg *xmlapi.RGXML
	for i := range summary.ResourceGroups {
		if summary.ResourceGroups[i].GroupName == "regtest-active-disable-rg" {
			rg = &summary.ResourceGroups[i]
		}
	}
	if rg == nil {
		t.Fatalf("resource group regtest-active-disable-rg not found in summary")
	}
	byName := map[string]xmlapi.ResourceXML{}
	for _, r := range rg.Resources.Resources {
		byName[r.Name] = r
	}

	cases := []struct {
		name        string
		wantActive  bool
		wantDisable bool
	}{
		{"res-both-omitted", true, false},
		{"res-active-false-disable-false", false, false},
		{"res-active-false-disable-true", false, true},
		{"res-active-true-disable-true", true, true},
	}
	for _, c := range cases {
		r, ok := byName[c.name]
		if !ok {
			t.Fatalf("resource %q not found in rendered summary", c.name)
		}
		if r.Active != c.wantActive {
			t.Errorf("%s: Active = %v, want %v", c.name, r.Active, c.wantActive)
		}
		if r.Disable != c.wantDisable {
			t.Errorf("%s: Disable = %v, want %v", c.name, r.Disable, c.wantDisable)
		}
	}
}
