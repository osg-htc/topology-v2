package handlers

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/testsupport"
	"github.com/bbockelm/topology-v2/internal/topology"
)

// TestApplyResourceProposal_RenameUpdatesInPlace guards the fix for the
// original reported bug: editing a resource's name via a proposal must update
// the existing row in place, never duplicate it, never silently move it to a
// different resource group, and never orphan its services/contacts.
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise, so `go test ./...` stays green without a database.
func TestApplyResourceProposal_RenameUpdatesInPlace(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run the resource-rename regression test")
	}

	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	const (
		topID     int64 = 900000001
		oldName         = "regtest-resource-old"
		newName         = "regtest-resource-new"
		groupName       = "regtest-group"
	)

	var rgID string
	var actorID string
	t.Run("seed", func(t *testing.T) {
		var err error
		actorID, err = q.CreateUser(ctx, db.CreateUserParams{
			DisplayName: "regtest-actor", Status: "active", IsProvisioned: true,
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}

		facID, err := q.InsertFacility(ctx, db.FacilityRow{
			TopologyID: 900000010, Name: "regtest-facility", IDExplicit: true,
		})
		if err != nil {
			t.Fatalf("InsertFacility: %v", err)
		}
		siteID, err := q.InsertSite(ctx, db.SiteRow{
			TopologyID: 900000011, FacilityID: facID, Name: "regtest-site", IDExplicit: true,
		})
		if err != nil {
			t.Fatalf("InsertSite: %v", err)
		}
		rgID, err = q.InsertResourceGroup(ctx, db.ResourceGroupRow{
			GroupID: 900000012, SiteID: siteID, Name: groupName, IDExplicit: true,
		})
		if err != nil {
			t.Fatalf("InsertResourceGroup: %v", err)
		}

		active := true
		if err := q.InsertResource(ctx, db.ResourceRow{
			TopologyID: topID, ResourceGroupID: rgID, Name: oldName,
			Active: &active, FQDN: "regtest.example.org", Description: "before rename",
			IDExplicit: true,
		}); err != nil {
			t.Fatalf("InsertResource: %v", err)
		}
		if err := q.InsertResourceService(ctx, db.ResourceServiceRow{
			ResourceID: topID, ServiceName: "CE", Description: "Compute Element",
		}); err != nil {
			t.Fatalf("InsertResourceService: %v", err)
		}
		if err := q.InsertResourceContact(ctx, db.ResourceContactRow{
			ResourceID: topID, ContactType: "Administrative Contact", Rank: "Primary",
			ContactName: "Test Contact",
		}); err != nil {
			t.Fatalf("InsertResourceContact: %v", err)
		}
	})

	var before []db.ResourceRow
	t.Run("capture before-state", func(t *testing.T) {
		rows, err := q.ListResources(ctx)
		if err != nil {
			t.Fatalf("ListResources: %v", err)
		}
		for _, r := range rows {
			if r.TopologyID == topID {
				before = append(before, r)
			}
		}
		if len(before) != 1 {
			t.Fatalf("expected exactly 1 seeded row at topology_id %d, got %d", topID, len(before))
		}
	})

	t.Run("apply rename", func(t *testing.T) {
		// Mirrors the real edit form: it prefills and resends the *full*
		// resource state (services/contacts included), changing only the
		// name -- the apply path replaces children from whatever the
		// proposal specifies, it doesn't merge with what's already there.
		active := true
		rp := resourceProposal{
			ResourceGroup: groupName,
			Name:          newName,
			Resource: topology.Resource{
				Active:      &active,
				FQDN:        "regtest.example.org",
				Description: "after rename",
				Services: map[string]*topology.Service{
					"CE": {Description: "Compute Element"},
				},
				ContactLists: map[string]map[string]topology.Contact{
					"Administrative Contact": {
						"Primary": {Name: "Test Contact"},
					},
				},
			},
		}
		state, err := json.Marshal(rp)
		if err != nil {
			t.Fatalf("marshal proposed_state: %v", err)
		}
		p := &models.Proposal{
			EntityKind:    models.KindResource,
			Operation:     models.OpUpdate,
			TargetName:    strconv.FormatInt(topID, 10),
			ProposedState: state,
		}
		h := &Handler{queries: q}
		if err := h.applyResourceProposal(ctx, q, p, actorID); err != nil {
			t.Fatalf("applyResourceProposal: %v", err)
		}
	})

	t.Run("assert no duplication, no silent group move", func(t *testing.T) {
		rows, err := q.ListResources(ctx)
		if err != nil {
			t.Fatalf("ListResources: %v", err)
		}
		var at []db.ResourceRow
		for _, r := range rows {
			if r.TopologyID == topID {
				at = append(at, r)
			}
			if r.Name == oldName {
				t.Fatalf("row still exists under the old name %q after rename: %+v", oldName, r)
			}
		}
		if len(at) != 1 {
			t.Fatalf("expected exactly 1 row at topology_id %d after rename, got %d: %+v", topID, len(at), at)
		}
		got := at[0]
		if got.Name != newName {
			t.Fatalf("name = %q, want %q", got.Name, newName)
		}

		// Full-row check: only Name and Description (the two fields the proposal
		// actually changed) may differ from the pre-rename row. Anything else
		// changing -- especially ResourceGroupID -- is a regression, since
		// UpdateResourceFromProposal writes resource_group_id unconditionally
		// from the resolved group and a bug there would silently move the
		// resource to a different group with no error.
		want := before[0]
		want.Name = newName
		want.Description = "after rename"
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("unexpected field changes beyond Name/Description.\nbefore: %+v\nafter:  %+v", before[0], got)
		}
	})

	t.Run("assert children preserved", func(t *testing.T) {
		svcs, err := q.ListResourceServices(ctx, topID)
		if err != nil {
			t.Fatalf("ListResourceServices: %v", err)
		}
		if len(svcs) != 1 || svcs[0].ServiceName != "CE" {
			t.Fatalf("resource_services not preserved across rename: %+v", svcs)
		}
		contacts, err := q.ListResourceContacts(ctx, topID)
		if err != nil {
			t.Fatalf("ListResourceContacts: %v", err)
		}
		if len(contacts) != 1 || contacts[0].ContactName != "Test Contact" {
			t.Fatalf("resource_contacts not preserved across rename: %+v", contacts)
		}
	})
}
