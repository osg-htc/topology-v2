package handlers

import (
	"context"
	"os"
	"testing"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// TestApplyFacilityProposal_DeleteBlockedByLiveSite guards a real bug: a
// Facility with a live Site could be soft-deleted with no check at all.
// BuildResourceSummary looks a resource group's Facility/Site up only among
// non-deleted rows (ListFacilities/ListSites both filter deleted_at IS
// NULL), so the orphaned Site's own resource groups would silently render
// with a blank, zero-valued Facility block in /rgsummary/xml -- the most-
// consumed feed -- instead of anyone seeing an error anywhere.
func TestApplyFacilityProposal_DeleteBlockedByLiveSite(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}

	actorID, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "regtest-actor", Status: "active", IsProvisioned: true})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	facID, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000500, Name: "regtest-delete-facility", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}
	if _, err := q.InsertSite(ctx, db.SiteRow{TopologyID: 900000501, FacilityID: facID, Name: "regtest-delete-site", IDExplicit: true}); err != nil {
		t.Fatalf("InsertSite: %v", err)
	}

	p := &models.Proposal{EntityKind: models.KindFacility, Operation: models.OpDelete, TargetName: "regtest-delete-facility"}
	if err := h.applyFacilityProposal(ctx, q, p, actorID); err == nil {
		t.Fatalf("applyFacilityProposal: expected deletion to be blocked by a live site, got success")
	}

	facs, err := q.ListFacilities(ctx)
	if err != nil {
		t.Fatalf("ListFacilities: %v", err)
	}
	for _, f := range facs {
		if f.Name == "regtest-delete-facility" {
			return // still present, as expected
		}
	}
	t.Fatalf("facility was deleted despite having a live site")
}

// TestApplyFacilityProposal_DeleteSucceedsWithNoChildren is the control case:
// a facility with no sites left must still be deletable.
func TestApplyFacilityProposal_DeleteSucceedsWithNoChildren(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}

	actorID, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "regtest-actor", Status: "active", IsProvisioned: true})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000502, Name: "regtest-childless-facility", IDExplicit: true}); err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}

	p := &models.Proposal{EntityKind: models.KindFacility, Operation: models.OpDelete, TargetName: "regtest-childless-facility"}
	if err := h.applyFacilityProposal(ctx, q, p, actorID); err != nil {
		t.Fatalf("applyFacilityProposal: expected a childless facility to delete cleanly, got: %v", err)
	}
	facs, err := q.ListFacilities(ctx)
	if err != nil {
		t.Fatalf("ListFacilities: %v", err)
	}
	for _, f := range facs {
		if f.Name == "regtest-childless-facility" {
			t.Fatalf("facility still present after a delete that should have succeeded")
		}
	}
}

// TestApplySiteProposal_DeleteBlockedByLiveResourceGroup mirrors the facility
// guard one level down: a site with a live resource group must not delete.
func TestApplySiteProposal_DeleteBlockedByLiveResourceGroup(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}

	actorID, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "regtest-actor", Status: "active", IsProvisioned: true})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	facID, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000503, Name: "regtest-site-delete-facility", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}
	siteID, err := q.InsertSite(ctx, db.SiteRow{TopologyID: 900000504, FacilityID: facID, Name: "regtest-site-delete-site", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertSite: %v", err)
	}
	if _, err := q.InsertResourceGroup(ctx, db.ResourceGroupRow{GroupID: 900000505, SiteID: siteID, Name: "regtest-site-delete-rg", IDExplicit: true}); err != nil {
		t.Fatalf("InsertResourceGroup: %v", err)
	}

	p := &models.Proposal{EntityKind: models.KindSite, Operation: models.OpDelete, TargetName: "regtest-site-delete-site"}
	if err := h.applySiteProposal(ctx, q, p, actorID); err == nil {
		t.Fatalf("applySiteProposal: expected deletion to be blocked by a live resource group, got success")
	}
}

// TestApplyResourceGroupProposal_DeleteBlockedByLiveResource mirrors the same
// guard one more level down: a resource group with a live resource must not
// delete.
func TestApplyResourceGroupProposal_DeleteBlockedByLiveResource(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}

	actorID, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "regtest-actor", Status: "active", IsProvisioned: true})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	facID, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000506, Name: "regtest-rg-delete-facility", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}
	siteID, err := q.InsertSite(ctx, db.SiteRow{TopologyID: 900000507, FacilityID: facID, Name: "regtest-rg-delete-site", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertSite: %v", err)
	}
	rgID, err := q.InsertResourceGroup(ctx, db.ResourceGroupRow{GroupID: 900000508, SiteID: siteID, Name: "regtest-rg-delete-rg", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertResourceGroup: %v", err)
	}
	if err := q.InsertResource(ctx, db.ResourceRow{
		TopologyID: 900000509, ResourceGroupID: rgID, Name: "regtest-rg-delete-resource",
		FQDN: "regtest-rg-delete.example.org", IDExplicit: true,
	}); err != nil {
		t.Fatalf("InsertResource: %v", err)
	}

	p := &models.Proposal{EntityKind: models.KindResourceGroup, Operation: models.OpDelete, TargetName: "regtest-rg-delete-rg"}
	if err := h.applyResourceGroupProposal(ctx, q, p, actorID); err == nil {
		t.Fatalf("applyResourceGroupProposal: expected deletion to be blocked by a live resource, got success")
	}
}
