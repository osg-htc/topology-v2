package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// withRouteParam attaches a chi route param the way the router would, for
// testing handlers that read chi.URLParam without going through chi's
// routing tree.
func withRouteParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// TestFileReplacement_RequesterContactID_NoCILogonIdentity guards the fix for
// primaryCILogonID: a dev-login user with no CILogon identity -- only an
// email-derived legacy_contact_id -- must still get a non-blank
// requester_contact_id when filing a replacement request. Before the fix,
// primaryCILogonID looked only at user_identities.cilogon_id and returned ""
// for such a user.
func TestFileReplacement_RequesterContactID_NoCILogonIdentity(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	pool, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}

	legacyID := emailSHA1("dev-login-requester@example.org")
	requesterID, err := q.CreateUser(ctx, db.CreateUserParams{
		DisplayName: "Dev Login Requester", Status: "active", LegacyContactID: legacyID,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// No user_identities row for requesterID -- the dev-login case: no
	// CILogon identity at all.

	if err := q.AddEntityContact(ctx, models.KindSite, "regtest-site", "Administrative Contact", "Primary",
		"Incumbent Person", emailSHA1("incumbent@example.org"), ""); err != nil {
		t.Fatalf("AddEntityContact (seed incumbent): %v", err)
	}

	id, herr := h.fileReplacement(asUser(requesterID), createReplacementRequest{
		EntityKind: models.KindSite, EntityName: "regtest-site",
		ContactType: "Administrative Contact", Rank: "Primary",
		RequesterUserID: requesterID,
	})
	if herr != nil {
		t.Fatalf("fileReplacement: %v", herr)
	}

	var gotContactID string
	if err := pool.QueryRow(ctx,
		"SELECT COALESCE(requester_contact_id,'') FROM contact_replacements WHERE id = $1", id).
		Scan(&gotContactID); err != nil {
		t.Fatalf("querying requester_contact_id: %v", err)
	}
	if gotContactID == "" {
		t.Fatalf("requester_contact_id is blank for a non-CILogon requester -- the exact bug")
	}
	if gotContactID != legacyID {
		t.Fatalf("requester_contact_id = %q, want %q (requester's LegacyContactID)", gotContactID, legacyID)
	}
}

// TestDecideReplacement_ApprovedHandoff_NoCILogonIdentity is the direct
// regression test for the live-reproduced bug: a dev-login (non-CILogon)
// user whose contact hand-off request is approved must end up with a
// non-blank contact id on the resulting slot, matching their
// legacy_contact_id.
func TestDecideReplacement_ApprovedHandoff_NoCILogonIdentity(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}

	legacyID := emailSHA1("handoff-requester@example.org")
	requesterID, err := q.CreateUser(ctx, db.CreateUserParams{
		DisplayName: "Handoff Requester", Status: "active", LegacyContactID: legacyID,
	})
	if err != nil {
		t.Fatalf("CreateUser (requester): %v", err)
	}
	incumbentID, err := q.CreateUser(ctx, db.CreateUserParams{
		DisplayName: "Incumbent", Status: "active", LegacyContactID: emailSHA1("incumbent2@example.org"),
	})
	if err != nil {
		t.Fatalf("CreateUser (incumbent): %v", err)
	}
	if err := q.AddEntityContact(ctx, models.KindSite, "regtest-handoff-site", "Administrative Contact", "Primary",
		"Incumbent", "", incumbentID); err != nil {
		t.Fatalf("AddEntityContact (seed incumbent): %v", err)
	}

	repID, herr := h.fileReplacement(asUser(requesterID), createReplacementRequest{
		EntityKind: models.KindSite, EntityName: "regtest-handoff-site",
		ContactType: "Administrative Contact", Rank: "Primary",
		RequesterUserID: requesterID,
	})
	if herr != nil {
		t.Fatalf("fileReplacement: %v", herr)
	}

	w := httptest.NewRecorder()
	r := withRouteParam(asUser(incumbentID), "id", repID)
	h.DecideReplacement(w, r, true)
	if w.Code != http.StatusOK {
		t.Fatalf("DecideReplacement approve: status = %d, body = %s", w.Code, w.Body.String())
	}

	contacts, err := q.ListEntityContacts(ctx, models.KindSite, "regtest-handoff-site")
	if err != nil {
		t.Fatalf("ListEntityContacts: %v", err)
	}
	if len(contacts) != 1 {
		t.Fatalf("expected exactly 1 active contact after approval, got %d: %+v", len(contacts), contacts)
	}
	if contacts[0].ID == "" {
		t.Fatalf("resulting contact id is blank for a non-CILogon requester -- the exact live-reproduced bug")
	}
	if contacts[0].ID != legacyID {
		t.Fatalf("resulting contact id = %q, want %q (requester's LegacyContactID)", contacts[0].ID, legacyID)
	}
}

// Note: a regression test for the new GetUser failure path in
// DecideReplacement (a requester deleted between filing and deciding) isn't
// constructible -- contact_replacements.requester_user_id has a NOT NULL FK
// to users(id) with no ON DELETE clause, so a requester referenced by any
// replacement request can never be hard-deleted. The 500-on-error handling
// (mirroring fileReplacement's own GetUser error handling) is still correct
// defensive practice against any other GetUser failure, just not one this
// schema lets a test reach.
