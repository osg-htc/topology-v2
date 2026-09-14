package xmlapi_test

import (
	"context"
	"os"
	"testing"

	"github.com/bbockelm/topology-v2/internal/testsupport"
	"github.com/bbockelm/topology-v2/internal/topology"
	"github.com/bbockelm/topology-v2/internal/xmlapi"
)

// TestBuildResourceSummary_ResourceGroupDefaults guards two ResourceGroup bugs
// of the same class as the already-fixed Resource.Active/Disable ones
// (docs/SPEC_CONFORMANCE_AUDIT.md, extended by a later parity pass):
//
//  1. Production's default was inverted: v1 treats an omitted Production as
//     false (ITB) -- `is_true(yaml_data.get("Production", ""))` -- but v2
//     treated nil as true (production).
//  2. Disable was hardcoded to false in the XML builder instead of being read
//     from real, independent data -- v1 defaults it to false but honors an
//     explicit override in the RG's own YAML (11 real resource groups set it
//     explicitly today, all to false).
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise, so `go test ./...` stays green without a database.
func TestBuildResourceSummary_ResourceGroupDefaults(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	tree := &topology.Topology{
		Facilities: map[string]*topology.Facility{
			"regtest-rg-defaults-facility": {Name: "regtest-rg-defaults-facility"},
		},
		Sites: map[string]*topology.Site{
			"regtest-rg-defaults-site": {Name: "regtest-rg-defaults-site"},
		},
		ResourceGroups: map[string]*topology.ResourceGroup{
			// Both omitted -- v1 default is Production=false (ITB), Disable=false.
			"rg-both-omitted": {Name: "rg-both-omitted", Resources: map[string]*topology.Resource{}},
			"rg-production-true-disable-false": {
				Name: "rg-production-true-disable-false", Resources: map[string]*topology.Resource{},
				Production: boolPtr(true), Disable: boolPtr(false),
			},
			// The exact live-bug pattern: independent fields, not derived.
			"rg-production-false-disable-true": {
				Name: "rg-production-false-disable-true", Resources: map[string]*topology.Resource{},
				Production: boolPtr(false), Disable: boolPtr(true),
			},
			"rg-production-true-disable-true": {
				Name: "rg-production-true-disable-true", Resources: map[string]*topology.Resource{},
				Production: boolPtr(true), Disable: boolPtr(true),
			},
		},
		Downtimes:    map[string][]*topology.Downtime{},
		SiteFacility: map[string]string{"regtest-rg-defaults-site": "regtest-rg-defaults-facility"},
		RGSite: map[string]string{
			"rg-both-omitted": "regtest-rg-defaults-site", "rg-production-true-disable-false": "regtest-rg-defaults-site",
			"rg-production-false-disable-true": "regtest-rg-defaults-site", "rg-production-true-disable-true": "regtest-rg-defaults-site",
		},
	}
	if err := topology.Import(ctx, q, tree); err != nil {
		t.Fatalf("Import: %v", err)
	}

	summary, err := xmlapi.BuildResourceSummary(ctx, q, nil, xmlapi.Filters{}, false)
	if err != nil {
		t.Fatalf("BuildResourceSummary: %v", err)
	}
	byName := map[string]xmlapi.RGXML{}
	for _, rg := range summary.ResourceGroups {
		byName[rg.GroupName] = rg
	}

	cases := []struct {
		name           string
		wantProduction bool
		wantDisable    bool
	}{
		{"rg-both-omitted", false, false},
		{"rg-production-true-disable-false", true, false},
		{"rg-production-false-disable-true", false, true},
		{"rg-production-true-disable-true", true, true},
	}
	for _, c := range cases {
		rg, ok := byName[c.name]
		if !ok {
			t.Fatalf("resource group %q not found in rendered summary", c.name)
		}
		if rg.Production != c.wantProduction {
			t.Errorf("%s: Production = %v, want %v", c.name, rg.Production, c.wantProduction)
		}
		if rg.Disable != c.wantDisable {
			t.Errorf("%s: Disable = %v, want %v", c.name, rg.Disable, c.wantDisable)
		}
	}
}
