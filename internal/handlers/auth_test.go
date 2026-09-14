package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// asRequestUser builds a request carrying a full *models.User, the way
// RequireAuth populates it (via GetUser) rather than asUser's bare-ID stand-in
// -- needed here because AcceptInvite reads u.LegacyContactID directly off
// the context user.
func asRequestUser(u *models.User, roles ...string) *http.Request {
	ctx := context.WithValue(context.Background(), ctxUser, u)
	ctx = context.WithValue(ctx, ctxRoles, roles)
	return httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
}

// TestAcceptInvite_RoleClaim_NoCILogonIdentity guards the fix in AcceptInvite's
// role_claim branch: a dev-login user with no CILogon identity accepting a
// role_claim invite must end up as a contact with a non-blank id, matching
// their legacy_contact_id -- not "" from the old CILogon-only lookup.
func TestAcceptInvite_RoleClaim_NoCILogonIdentity(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}

	legacyID := emailSHA1("invite-acceptor@example.org")
	userID, err := q.CreateUser(ctx, db.CreateUserParams{
		DisplayName: "Invite Acceptor", Status: "active", LegacyContactID: legacyID,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// No user_identities row -- dev-login user, no CILogon identity.
	u, err := q.GetUser(ctx, userID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}

	claim := models.RoleClaim{
		EntityKind: models.KindFacility, EntityID: "regtest-claim-facility",
		ContactType: "Administrative Contact", Rank: "Primary",
	}
	claimJSON, err := json.Marshal(claim)
	if err != nil {
		t.Fatalf("marshal claim: %v", err)
	}
	rawToken := "regtest-role-claim-token"
	if _, err := q.CreateInvite(ctx, db.CreateInviteParams{
		Kind: models.InviteRoleClaim, TokenHash: hashToken(rawToken),
		ClaimJSON: claimJSON, ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	w := httptest.NewRecorder()
	r := withRouteParam(asRequestUser(u), "token", rawToken)
	h.AcceptInvite(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("AcceptInvite: status = %d, body = %s", w.Code, w.Body.String())
	}

	contacts, err := q.ListEntityContacts(ctx, models.KindFacility, "regtest-claim-facility")
	if err != nil {
		t.Fatalf("ListEntityContacts: %v", err)
	}
	if len(contacts) != 1 {
		t.Fatalf("expected exactly 1 active contact after accepting, got %d: %+v", len(contacts), contacts)
	}
	if contacts[0].ID == "" {
		t.Fatalf("resulting contact id is blank for a non-CILogon accepting user -- the exact bug")
	}
	if contacts[0].ID != legacyID {
		t.Fatalf("resulting contact id = %q, want %q (accepting user's LegacyContactID)", contacts[0].ID, legacyID)
	}
}
