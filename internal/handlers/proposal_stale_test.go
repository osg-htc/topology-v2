package handlers

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/testsupport"
	"github.com/bbockelm/topology-v2/internal/topology"
)

// TestEntityUpdatedAt covers the TEMPORARY stale-base guard's core lookup
// (proposal_stale.go): every supported kind resolves to its live
// updated_at, and every edge case that must resolve to "stale" (nil, not an
// error) actually does -- an empty target, an unsupported kind, a not-found
// row (including a soft-deleted or never-existed entity, which is meant to
// be caught as stale too), and a non-numeric resource target.
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise, so `go test ./...` stays green without a database.
func TestEntityUpdatedAt(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run the stale-base guard tests")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	const (
		facName   = "regtest-stale-facility"
		facTopID  = int64(900000030)
		resTopID  = int64(900000031)
		groupName = "regtest-stale-group"
	)
	facID, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: facTopID, Name: facName, IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}
	siteID, err := q.InsertSite(ctx, db.SiteRow{TopologyID: 900000032, FacilityID: facID, Name: "regtest-stale-site", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertSite: %v", err)
	}
	rgID, err := q.InsertResourceGroup(ctx, db.ResourceGroupRow{GroupID: 900000033, SiteID: siteID, Name: groupName, IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertResourceGroup: %v", err)
	}
	active := true
	if err := q.InsertResource(ctx, db.ResourceRow{
		TopologyID: resTopID, ResourceGroupID: rgID, Name: "regtest-stale-resource",
		Active: &active, FQDN: "regtest-stale.example.org", IDExplicit: true,
	}); err != nil {
		t.Fatalf("InsertResource: %v", err)
	}

	t.Run("facility resolves to its live updated_at", func(t *testing.T) {
		got, err := entityUpdatedAt(ctx, q, models.KindFacility, facName)
		if err != nil {
			t.Fatalf("entityUpdatedAt: %v", err)
		}
		if got == nil {
			t.Fatal("got nil, want a real timestamp")
		}
		if time.Since(*got) > time.Minute {
			t.Errorf("updated_at = %v, want close to now (just inserted)", got)
		}
	})

	t.Run("resource resolves to its live updated_at", func(t *testing.T) {
		got, err := entityUpdatedAt(ctx, q, models.KindResource, strconv.FormatInt(resTopID, 10))
		if err != nil {
			t.Fatalf("entityUpdatedAt: %v", err)
		}
		if got == nil {
			t.Fatal("got nil, want a real timestamp")
		}
	})

	t.Run("a live update bumps what entityUpdatedAt reports", func(t *testing.T) {
		before, err := entityUpdatedAt(ctx, q, models.KindFacility, facName)
		if err != nil || before == nil {
			t.Fatalf("entityUpdatedAt (before): got (%v, %v)", before, err)
		}
		if err := q.UpdateFacilityFields(ctx, facName, facName, "some-institution"); err != nil {
			t.Fatalf("UpdateFacilityFields: %v", err)
		}
		after, err := entityUpdatedAt(ctx, q, models.KindFacility, facName)
		if err != nil || after == nil {
			t.Fatalf("entityUpdatedAt (after): got (%v, %v)", after, err)
		}
		if !after.After(*before) && !after.Equal(*before) {
			t.Errorf("updated_at went backwards after a live update: before=%v after=%v", before, after)
		}
		if after.Equal(*before) {
			t.Error("updated_at did not change after a live update -- the guard would never detect this as stale")
		}
	})

	t.Run("empty target resolves to stale (nil, no error)", func(t *testing.T) {
		got, err := entityUpdatedAt(ctx, q, models.KindFacility, "")
		if err != nil || got != nil {
			t.Errorf("got (%v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("unsupported kind resolves to stale (nil, no error)", func(t *testing.T) {
		got, err := entityUpdatedAt(ctx, q, models.KindDowntime, "1")
		if err != nil || got != nil {
			t.Errorf("got (%v, %v), want (nil, nil)", got, err)
		}
		got, err = entityUpdatedAt(ctx, q, models.KindBundle, "1")
		if err != nil || got != nil {
			t.Errorf("got (%v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("not-found entity resolves to stale (nil, no error), not an error", func(t *testing.T) {
		// Catches "the entity was deleted out from under this proposal" as a
		// side effect -- deliberately not an error, since the caller treats
		// nil as "compare unequal, refuse to apply" either way.
		got, err := entityUpdatedAt(ctx, q, models.KindFacility, "does-not-exist")
		if err != nil || got != nil {
			t.Errorf("got (%v, %v), want (nil, nil)", got, err)
		}
	})

	t.Run("resource with a non-numeric target resolves to stale (nil, no error)", func(t *testing.T) {
		got, err := entityUpdatedAt(ctx, q, models.KindResource, "not-a-number")
		if err != nil || got != nil {
			t.Errorf("got (%v, %v), want (nil, nil)", got, err)
		}
	})
}

// TestApplyProposal_StaleBaseGuard exercises the guard end to end through
// applyProposal (not just entityUpdatedAt in isolation): a proposal whose
// captured BaseUpdatedAt no longer matches the live entity must be refused
// with errStaleBase and must NOT partially apply; a proposal whose
// BaseUpdatedAt still matches must apply normally.
func TestApplyProposal_StaleBaseGuard(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run the stale-base guard tests")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}

	const (
		topID     = int64(900000040)
		groupName = "regtest-guard-group"
	)
	facID, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000041, Name: "regtest-guard-facility", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}
	siteID, err := q.InsertSite(ctx, db.SiteRow{TopologyID: 900000042, FacilityID: facID, Name: "regtest-guard-site", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertSite: %v", err)
	}
	rgID, err := q.InsertResourceGroup(ctx, db.ResourceGroupRow{GroupID: 900000043, SiteID: siteID, Name: groupName, IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertResourceGroup: %v", err)
	}
	active := true
	if err := q.InsertResource(ctx, db.ResourceRow{
		TopologyID: topID, ResourceGroupID: rgID, Name: "regtest-guard-resource",
		Active: &active, FQDN: "regtest-guard.example.org", IDExplicit: true,
	}); err != nil {
		t.Fatalf("InsertResource: %v", err)
	}

	staleBase, err := entityUpdatedAt(ctx, q, models.KindResource, strconv.FormatInt(topID, 10))
	if err != nil || staleBase == nil {
		t.Fatalf("capturing base updated_at: got (%v, %v)", staleBase, err)
	}

	proposedState := func(desc string) json.RawMessage {
		rp := resourceProposal{
			ResourceGroup: groupName, Name: "regtest-guard-resource",
			Resource: topology.Resource{
				Active: &active, FQDN: "regtest-guard.example.org", Description: desc,
				// The schema requires these as arrays/objects, never null --
				// a nil field here would fail schema validation for a
				// reason unrelated to what this test actually checks.
				FQDNAliases: []string{}, Tags: []string{}, AllowedVOs: []string{},
				Services:     map[string]*topology.Service{},
				ContactLists: map[string]map[string]topology.Contact{},
			},
		}
		b, err := json.Marshal(rp)
		if err != nil {
			t.Fatalf("marshal proposed_state: %v", err)
		}
		return b
	}

	t.Run("stale base is refused, and does not partially apply", func(t *testing.T) {
		// Someone else's change lands first, bumping updated_at.
		if err := q.UpdateResourceFields(ctx, db.ResourceRow{
			TopologyID: topID, ResourceGroupID: rgID, Name: "regtest-guard-resource",
			Active: &active, FQDN: "regtest-guard.example.org", Description: "someone else's change",
		}); err != nil {
			t.Fatalf("UpdateResourceFields: %v", err)
		}

		p := &models.Proposal{
			EntityKind: models.KindResource, Operation: models.OpUpdate,
			TargetName: strconv.FormatInt(topID, 10), ProposedState: proposedState("my stale edit"),
			BaseUpdatedAt: staleBase,
		}
		err := h.applyProposal(ctx, q, p, "")
		if err != errStaleBase {
			t.Fatalf("got %v, want errStaleBase", err)
		}

		row, err := q.GetResourceRow(ctx, topID)
		if err != nil {
			t.Fatalf("GetResourceRow: %v", err)
		}
		if row.Description != "someone else's change" {
			t.Errorf("a refused apply must not partially write: description = %q, want the other change untouched", row.Description)
		}
	})

	t.Run("a fresh base applies normally", func(t *testing.T) {
		fresh, err := entityUpdatedAt(ctx, q, models.KindResource, strconv.FormatInt(topID, 10))
		if err != nil || fresh == nil {
			t.Fatalf("capturing fresh updated_at: got (%v, %v)", fresh, err)
		}
		p := &models.Proposal{
			EntityKind: models.KindResource, Operation: models.OpUpdate,
			TargetName: strconv.FormatInt(topID, 10), ProposedState: proposedState("a fresh edit"),
			BaseUpdatedAt: fresh,
		}
		if err := h.applyProposal(ctx, q, p, ""); err != nil {
			t.Fatalf("applyProposal with a fresh base: %v", err)
		}
		row, err := q.GetResourceRow(ctx, topID)
		if err != nil {
			t.Fatalf("GetResourceRow: %v", err)
		}
		if row.Description != "a fresh edit" {
			t.Errorf("description = %q, want the fresh edit to have applied", row.Description)
		}
	})
}
