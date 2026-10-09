package handlers

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/testsupport"
)

func facilityProposalFor(op, target, state string) *models.Proposal {
	return &models.Proposal{
		EntityKind: models.KindFacility, Operation: op,
		TargetName: target, ProposedState: []byte(state),
	}
}

func facilityInstitution(t *testing.T, q *db.Queries, name string) string {
	t.Helper()
	row, err := q.GetFacilityRow(context.Background(), name)
	if err != nil {
		t.Fatalf("GetFacilityRow(%q): %v", name, err)
	}
	return row.InstitutionID
}

// TestApplyFacilityProposal_UpdateKeepsMissingInstitution guards a real bug:
// applyFacilityProposal demanded an institution_id on every operation, so a
// live facility with none on record (v1 tolerates InstitutionID: null --
// Gridplexus and NSF DC are real examples) could never be edited at all: no
// rename, no contact change, nothing.
func TestApplyFacilityProposal_UpdateKeepsMissingInstitution(t *testing.T) {
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
	if _, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000600, Name: "regtest-no-inst-facility", IDExplicit: true}); err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}

	p := facilityProposalFor(models.OpUpdate, "regtest-no-inst-facility",
		`{"name":"regtest-no-inst-facility-renamed","institution_id":"","contacts":[]}`)
	if err := h.applyFacilityProposal(ctx, q, p, actorID); err != nil {
		t.Fatalf("editing a facility that has no institution must succeed, got: %v", err)
	}
	if got := facilityInstitution(t, q, "regtest-no-inst-facility-renamed"); got != "" {
		t.Fatalf("institution should stay unset, got %q", got)
	}
}

// A facility with no institution may also be given one on edit.
func TestApplyFacilityProposal_UpdateCanSetInstitutionOnFacilityWithNone(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}
	actorID, _ := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "regtest-actor", Status: "active", IsProvisioned: true})
	if err := q.UpsertInstitution(ctx, "https://osg-htc.org/iid/regtest01", "Regtest University", ""); err != nil {
		t.Fatalf("UpsertInstitution: %v", err)
	}
	if _, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000601, Name: "regtest-gain-inst-facility", IDExplicit: true}); err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}
	p := facilityProposalFor(models.OpUpdate, "regtest-gain-inst-facility",
		`{"name":"regtest-gain-inst-facility","institution_id":"https://osg-htc.org/iid/regtest01","contacts":[]}`)
	if err := h.applyFacilityProposal(ctx, q, p, actorID); err != nil {
		t.Fatalf("setting an institution on a facility that had none must succeed, got: %v", err)
	}
	if got := facilityInstitution(t, q, "regtest-gain-inst-facility"); got != "https://osg-htc.org/iid/regtest01" {
		t.Fatalf("institution = %q, want the registry id", got)
	}
}

// The requirement must still hold where it did: dropping the institution from
// a facility that has one, and creating a facility with none, are both rejected.
func TestApplyFacilityProposal_InstitutionStillRequiredWhereItWas(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}
	actorID, _ := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "regtest-actor", Status: "active", IsProvisioned: true})
	if err := q.UpsertInstitution(ctx, "https://osg-htc.org/iid/regtest02", "Regtest College", ""); err != nil {
		t.Fatalf("UpsertInstitution: %v", err)
	}
	if _, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000602, Name: "regtest-has-inst-facility",
		InstitutionID: "https://osg-htc.org/iid/regtest02", IDExplicit: true}); err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}

	// 1. Clearing the institution of a facility that has one is rejected, and nothing changes.
	p := facilityProposalFor(models.OpUpdate, "regtest-has-inst-facility",
		`{"name":"regtest-has-inst-facility","institution_id":"","contacts":[]}`)
	err := h.applyFacilityProposal(ctx, q, p, actorID)
	if err == nil || !strings.Contains(err.Error(), "requires an institution") {
		t.Fatalf("dropping an existing institution must be rejected, got: %v", err)
	}
	if got := facilityInstitution(t, q, "regtest-has-inst-facility"); got != "https://osg-htc.org/iid/regtest02" {
		t.Fatalf("institution changed to %q despite rejection", got)
	}

	// 2. Creating a facility with no institution is still rejected.
	p = facilityProposalFor(models.OpCreate, "", `{"name":"regtest-new-no-inst","institution_id":"","contacts":[]}`)
	if err := h.applyFacilityProposal(ctx, q, p, actorID); err == nil || !strings.Contains(err.Error(), "requires an institution") {
		t.Fatalf("creating a facility without an institution must be rejected, got: %v", err)
	}

	// 3. An unknown institution is still rejected, even on a facility that had none.
	if _, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000603, Name: "regtest-bogus-inst-facility", IDExplicit: true}); err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}
	p = facilityProposalFor(models.OpUpdate, "regtest-bogus-inst-facility",
		`{"name":"regtest-bogus-inst-facility","institution_id":"https://osg-htc.org/iid/nope","contacts":[]}`)
	if err := h.applyFacilityProposal(ctx, q, p, actorID); err == nil || !strings.Contains(err.Error(), "not in the registry") {
		t.Fatalf("an unregistered institution must be rejected, got: %v", err)
	}
}
