package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/testsupport"
	"github.com/bbockelm/topology-v2/internal/topology"
)

// asUser builds a request carrying the same context values RequireAuth
// installs, for testing handler methods that read currentUser/rolesFromContext
// without going through the real HTTP/middleware stack.
func asUser(userID string, roles ...string) *http.Request {
	ctx := context.WithValue(context.Background(), ctxUser, &models.User{ID: userID})
	ctx = context.WithValue(ctx, ctxRoles, roles)
	return httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
}

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

	contactID := emailSHA1("test-contact@example.org")
	t.Run("apply rename", func(t *testing.T) {
		// A contact must resolve to a real user (see requireResolvedContacts) --
		// seed one, keyed by the same legacy-contact-id scheme, to stand in
		// for "Test Contact" being picked in the form.
		if _, err := q.CreateUser(ctx, db.CreateUserParams{
			DisplayName: "Test Contact", Status: "active", IsProvisioned: true, LegacyContactID: contactID,
		}); err != nil {
			t.Fatalf("CreateUser (contact): %v", err)
		}
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
						"Primary": {Name: "Test Contact", ID: contactID},
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
		if contacts[0].ContactID != contactID {
			t.Fatalf("ListResourceContacts ContactID = %q, want %q", contacts[0].ContactID, contactID)
		}
	})
}

// TestApplyFacilityProposal_ContactMustResolveToRealUser guards the core fix:
// a contact with no ID, or an ID that doesn't match a real user's
// legacy_contact_id, must reject the apply -- mirroring how a bad
// institution_id already does. requireResolvedContacts is shared verbatim by
// applyResourceGroupProposal/applySiteProposal/applyFacilityProposal, so
// covering facility here covers the same code path for the other two.
func TestApplyFacilityProposal_ContactMustResolveToRealUser(t *testing.T) {
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
	if err := q.UpsertInstitution(ctx, "https://example.org/iid/regtest", "Regtest University", ""); err != nil {
		t.Fatalf("UpsertInstitution: %v", err)
	}

	marshal := func(v any) json.RawMessage {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal fixture: %v", err)
		}
		return b
	}

	t.Run("missing ID is rejected", func(t *testing.T) {
		p := &models.Proposal{
			EntityKind: models.KindFacility, Operation: models.OpCreate,
			ProposedState: marshal(map[string]any{
				"name": "regtest-fac-missing-user", "institution_id": "https://example.org/iid/regtest",
				"contacts": []map[string]any{{"contact_type": "Administrative Contact", "name": "Nobody"}},
			}),
		}
		err := h.applyFacilityProposal(ctx, q, p, actorID)
		if err == nil {
			t.Fatalf("applyFacilityProposal: expected rejection for a contact with no ID, got success")
		}
	})

	t.Run("nonexistent ID is rejected", func(t *testing.T) {
		p := &models.Proposal{
			EntityKind: models.KindFacility, Operation: models.OpCreate,
			ProposedState: marshal(map[string]any{
				"name": "regtest-fac-bad-user", "institution_id": "https://example.org/iid/regtest",
				"contacts": []map[string]any{{"contact_type": "Administrative Contact", "name": "Ghost", "id": "no-such-legacy-id"}},
			}),
		}
		err := h.applyFacilityProposal(ctx, q, p, actorID)
		if err == nil {
			t.Fatalf("applyFacilityProposal: expected rejection for a nonexistent ID, got success")
		}
	})

	t.Run("real ID is accepted", func(t *testing.T) {
		contactID := emailSHA1("real-person@example.org")
		if _, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "Real Person", Status: "active", LegacyContactID: contactID}); err != nil {
			t.Fatalf("CreateUser (contact): %v", err)
		}
		p := &models.Proposal{
			EntityKind: models.KindFacility, Operation: models.OpCreate,
			ProposedState: marshal(map[string]any{
				"name": "regtest-fac-good-user", "institution_id": "https://example.org/iid/regtest",
				"contacts": []map[string]any{{"contact_type": "Administrative Contact", "name": "Real Person", "id": contactID}},
			}),
		}
		if err := h.applyFacilityProposal(ctx, q, p, actorID); err != nil {
			t.Fatalf("applyFacilityProposal: expected a resolved contact to be accepted, got: %v", err)
		}
	})

	// Regression guard: facility/site/resource_group contacts are a plain
	// ordered list with rank *derived* from position (rankForOrder clamps
	// anything past the 3rd to "Tertiary"), unlike a resource's ContactLists
	// (map[type][rank]Contact, structurally immune). A 4th same-type contact
	// must be rejected, not silently land on the same slot as the 3rd.
	t.Run("a 4th contact of the same type is rejected", func(t *testing.T) {
		contactID := emailSHA1("fourth-contact@example.org")
		if _, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "Fourth Contact", Status: "active", LegacyContactID: contactID}); err != nil {
			t.Fatalf("CreateUser (contact): %v", err)
		}
		contact := map[string]any{"contact_type": "Administrative Contact", "name": "Fourth Contact", "id": contactID}
		p := &models.Proposal{
			EntityKind: models.KindFacility, Operation: models.OpCreate,
			ProposedState: marshal(map[string]any{
				"name": "regtest-fac-fourth-contact", "institution_id": "https://example.org/iid/regtest",
				"contacts": []map[string]any{contact, contact, contact, contact},
			}),
		}
		if err := h.applyFacilityProposal(ctx, q, p, actorID); err == nil {
			t.Fatalf("applyFacilityProposal: expected rejection for a 4th same-type contact, got success")
		}
	})
}

// TestApplyResourceProposal_ContactRankValidation covers
// requireResolvedResourceContacts' two checks specific to the resource
// ContactLists map shape: an out-of-range/invalid rank key is rejected (the
// only backend-enforceable half of the resource form's same-type collision
// guard -- see buildResource() in proposals/new/page.tsx), and a resolved
// contact under a valid rank is accepted.
func TestApplyResourceProposal_ContactRankValidation(t *testing.T) {
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
	facID, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000070, Name: "regtest-rank-facility", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}
	siteID, err := q.InsertSite(ctx, db.SiteRow{TopologyID: 900000071, FacilityID: facID, Name: "regtest-rank-site", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertSite: %v", err)
	}
	if _, err := q.InsertResourceGroup(ctx, db.ResourceGroupRow{GroupID: 900000072, SiteID: siteID, Name: "regtest-rank-rg", IDExplicit: true}); err != nil {
		t.Fatalf("InsertResourceGroup: %v", err)
	}

	actorLegacyID := emailSHA1("regtest-actor@example.org")
	if err := q.SetLegacyContactIDIfMissing(ctx, actorID, actorLegacyID); err != nil {
		t.Fatalf("SetLegacyContactIDIfMissing: %v", err)
	}

	t.Run("invalid rank key is rejected", func(t *testing.T) {
		rp := resourceProposal{
			ResourceGroup: "regtest-rank-rg", Name: "regtest-rank-res-bad",
			Resource: topology.Resource{
				FQDN: "regtest-rank-bad.example.org",
				ContactLists: map[string]map[string]topology.Contact{
					"Administrative Contact": {"Quaternary": {Name: "Nobody", ID: actorLegacyID}},
				},
			},
		}
		state, err := json.Marshal(rp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		p := &models.Proposal{EntityKind: models.KindResource, Operation: models.OpCreate, ProposedState: state}
		if err := h.applyResourceProposal(ctx, q, p, actorID); err == nil {
			t.Fatalf("applyResourceProposal: expected rejection for rank %q, got success", "Quaternary")
		}
	})

	t.Run("valid rank with a resolved contact is accepted", func(t *testing.T) {
		rp := resourceProposal{
			ResourceGroup: "regtest-rank-rg", Name: "regtest-rank-res-good",
			Resource: topology.Resource{
				FQDN: "regtest-rank-good.example.org",
				ContactLists: map[string]map[string]topology.Contact{
					"Administrative Contact": {"Primary": {Name: "regtest-actor", ID: actorLegacyID}},
				},
			},
		}
		state, err := json.Marshal(rp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		p := &models.Proposal{EntityKind: models.KindResource, Operation: models.OpCreate, ProposedState: state}
		if err := h.applyResourceProposal(ctx, q, p, actorID); err != nil {
			t.Fatalf("applyResourceProposal: expected a resolved contact under a valid rank to be accepted, got: %v", err)
		}
	})
}

// TestApplyBundleProposal_ResourceGroupAndResourceInOneBundle guards a real
// bug: a bundle's synthetic per-operation *models.Proposal (built by
// applyBundleProposal) carries no id of its own, since there's no
// change_proposals row backing it. applyResourceProposal's post-create
// target_name backfill used that id unconditionally, so any bundle creating
// a resource (e.g. an inline parent chain: facility -> site -> resource
// group -> resource, exactly what the "Register a resource" form submits
// when the user creates new parents inline) failed with a Postgres "invalid
// input syntax for type uuid" error on every single approval -- caught via
// the e2e suite's bundle.spec.ts.
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise, so `go test ./...` stays green without a database.
func TestApplyBundleProposal_ResourceGroupAndResourceInOneBundle(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run the bundle regression test")
	}

	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}

	actorID, err := q.CreateUser(ctx, db.CreateUserParams{
		DisplayName: "regtest-bundle-actor", Status: "active", IsProvisioned: true,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	facID, err := q.InsertFacility(ctx, db.FacilityRow{
		TopologyID: 900000020, Name: "regtest-bundle-facility", IDExplicit: true,
	})
	if err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}
	if _, err := q.InsertSite(ctx, db.SiteRow{
		TopologyID: 900000021, FacilityID: facID, Name: "regtest-bundle-site", IDExplicit: true,
	}); err != nil {
		t.Fatalf("InsertSite: %v", err)
	}

	// A bundle whose resource operation targets a resource group created by
	// an earlier operation in the SAME bundle -- the exact shape the
	// "Register a resource" form submits when the user creates a new
	// resource group inline.
	marshal := func(v any) json.RawMessage {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal fixture: %v", err)
		}
		return b
	}
	bp := bundleProposal{Operations: []bundleOp{
		{EntityKind: models.KindResourceGroup, Operation: models.OpCreate, ProposedState: marshal(map[string]any{
			"name": "regtest-bundle-rg", "site": "regtest-bundle-site",
		})},
		{EntityKind: models.KindResource, Operation: models.OpCreate, ProposedState: marshal(map[string]any{
			"name": "regtest-bundle-resource", "resource_group": "regtest-bundle-rg",
			"resource": map[string]any{"Active": true, "FQDN": "regtest-bundle.example.org"},
		})},
	}}
	state, err := json.Marshal(bp)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	p := &models.Proposal{ID: "regtest-bundle-proposal-id", EntityKind: models.KindBundle, Operation: models.OpCreate, ProposedState: state}

	if err := q.WithTx(ctx, func(tx *db.Queries) error {
		return h.applyBundleProposal(ctx, tx, p, actorID)
	}); err != nil {
		t.Fatalf("applyBundleProposal: %v", err)
	}

	if _, err := q.ResourceGroupIDByName(ctx, "regtest-bundle-rg"); err != nil {
		t.Fatalf("resource group was not created: %v", err)
	}
	rows, err := q.ListResources(ctx)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	var found *db.ResourceRow
	for i := range rows {
		if rows[i].Name == "regtest-bundle-resource" {
			found = &rows[i]
		}
	}
	if found == nil {
		t.Fatalf("resource was not created")
	}
	if found.RGName != "regtest-bundle-rg" {
		t.Fatalf("resource's resource group = %q, want the bundle's own new group %q", found.RGName, "regtest-bundle-rg")
	}
}

// TestCanDecideProposal_DowntimeContactCarveOut guards the v1-parity fix:
// automerge_check.py grants a downtime change the same authority as being
// one of the affected resource's own contacts, no manager needed. Covers the
// resource's own contacts, inherited resource_group contacts, an unrelated
// user getting no carve-out, an update/delete proposal resolving its
// resource by dt_id rather than proposed_state, and non-downtime kinds never
// getting the carve-out at all.
func TestCanDecideProposal_DowntimeContactCarveOut(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}

	facID, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000080, Name: "regtest-dt-facility", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}
	siteID, err := q.InsertSite(ctx, db.SiteRow{TopologyID: 900000081, FacilityID: facID, Name: "regtest-dt-site", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertSite: %v", err)
	}
	rgID, err := q.InsertResourceGroup(ctx, db.ResourceGroupRow{GroupID: 900000082, SiteID: siteID, Name: "regtest-dt-rg", IDExplicit: true})
	if err != nil {
		t.Fatalf("InsertResourceGroup: %v", err)
	}

	const ownContactResource = "regtest-dt-resource-own"
	const inheritedContactResource = "regtest-dt-resource-inherited"
	if err := q.InsertResource(ctx, db.ResourceRow{
		TopologyID: 900000083, ResourceGroupID: rgID, Name: ownContactResource, FQDN: "regtest-dt-own.example.org", IDExplicit: true,
	}); err != nil {
		t.Fatalf("InsertResource (own): %v", err)
	}
	if err := q.InsertResource(ctx, db.ResourceRow{
		TopologyID: 900000084, ResourceGroupID: rgID, Name: inheritedContactResource, FQDN: "regtest-dt-inherited.example.org", IDExplicit: true,
	}); err != nil {
		t.Fatalf("InsertResource (inherited): %v", err)
	}

	ownContactID := emailSHA1("own-contact@example.org")
	ownContactUserID, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "Own Contact", Status: "active", LegacyContactID: ownContactID})
	if err != nil {
		t.Fatalf("CreateUser (own contact): %v", err)
	}
	if err := q.InsertResourceContact(ctx, db.ResourceContactRow{
		ResourceID: 900000083, ContactType: "Administrative Contact", Rank: "Primary",
		ContactName: "Own Contact", ContactID: ownContactID,
	}); err != nil {
		t.Fatalf("InsertResourceContact: %v", err)
	}

	rgContactID := emailSHA1("rg-contact@example.org")
	rgContactUserID, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "RG Contact", Status: "active", LegacyContactID: rgContactID})
	if err != nil {
		t.Fatalf("CreateUser (rg contact): %v", err)
	}
	if err := q.ReplaceEntityContacts(ctx, models.KindResourceGroup, "regtest-dt-rg", "regtest-dt-rg",
		[]db.EntityContact{{ContactType: "Administrative Contact", Name: "RG Contact", ID: rgContactID}}, ""); err != nil {
		t.Fatalf("ReplaceEntityContacts: %v", err)
	}

	otherUserID, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "Unrelated User", Status: "active"})
	if err != nil {
		t.Fatalf("CreateUser (unrelated): %v", err)
	}

	marshalDowntime := func(resource string) json.RawMessage {
		b, err := json.Marshal(map[string]any{"resource": resource, "class": "SCHEDULED", "start_time": "", "end_time": ""})
		if err != nil {
			t.Fatalf("marshal fixture: %v", err)
		}
		return b
	}

	t.Run("the resource's own contact may decide", func(t *testing.T) {
		p := &models.Proposal{EntityKind: models.KindDowntime, Operation: models.OpCreate, ProposedState: marshalDowntime(ownContactResource)}
		if !h.canDecideProposal(ctx, asUser(ownContactUserID, models.RoleUser), p) {
			t.Fatalf("expected the resource's own contact to be able to decide this downtime")
		}
	})

	t.Run("a contact inherited from the resource group may decide", func(t *testing.T) {
		p := &models.Proposal{EntityKind: models.KindDowntime, Operation: models.OpCreate, ProposedState: marshalDowntime(inheritedContactResource)}
		if !h.canDecideProposal(ctx, asUser(rgContactUserID, models.RoleUser), p) {
			t.Fatalf("expected a resource-group-inherited contact to be able to decide this downtime")
		}
	})

	t.Run("an unrelated user may not decide", func(t *testing.T) {
		p := &models.Proposal{EntityKind: models.KindDowntime, Operation: models.OpCreate, ProposedState: marshalDowntime(ownContactResource)}
		if h.canDecideProposal(ctx, asUser(otherUserID, models.RoleUser), p) {
			t.Fatalf("expected an unrelated user to be rejected")
		}
	})

	t.Run("an update proposal resolves its resource by dt_id, not proposed_state", func(t *testing.T) {
		const dtID int64 = 900000085
		if err := q.InsertDowntime(ctx, db.DowntimeRow{
			DtID: dtID, RGID: rgID, ResourceName: ownContactResource, Class: "SCHEDULED",
			StartTime: "x", EndTime: "y", CreatedTime: "z",
		}); err != nil {
			t.Fatalf("InsertDowntime: %v", err)
		}
		p := &models.Proposal{EntityKind: models.KindDowntime, Operation: models.OpUpdate, TargetName: strconv.FormatInt(dtID, 10)}
		if !h.canDecideProposal(ctx, asUser(ownContactUserID, models.RoleUser), p) {
			t.Fatalf("expected the resource's own contact to decide an update proposal resolved via dt_id")
		}
		if h.canDecideProposal(ctx, asUser(otherUserID, models.RoleUser), p) {
			t.Fatalf("expected an unrelated user to be rejected for the same update proposal")
		}
	})

	t.Run("non-downtime kinds never get the carve-out", func(t *testing.T) {
		p := &models.Proposal{EntityKind: models.KindFacility, Operation: models.OpUpdate, TargetName: "regtest-dt-facility"}
		if h.canDecideProposal(ctx, asUser(ownContactUserID, models.RoleUser), p) {
			t.Fatalf("expected a resource contact to get no carve-out on a non-downtime proposal")
		}
	})

	t.Run("a manager may always decide, contact or not", func(t *testing.T) {
		p := &models.Proposal{EntityKind: models.KindDowntime, Operation: models.OpCreate, ProposedState: marshalDowntime(ownContactResource)}
		if !h.canDecideProposal(ctx, asUser(otherUserID, models.RoleManager), p) {
			t.Fatalf("expected a manager to be able to decide any downtime")
		}
	})
}
