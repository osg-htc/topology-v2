package db_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// TestRepointEntityNameReferences guards the fix for a real bug this
// codebase already had once: a pending contact hand-off or invite stores a
// snapshot of the entity's name at creation time, so renaming the entity
// would silently orphan it under the stale name. This must follow a rename
// -- but only for requests that are still live; an already-decided
// replacement or an already-used invite is history and must be left alone.
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise.
func TestRepointEntityNameReferences(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)
	requester := newTestUser(t, ctx, q)

	const oldName, newName, kind = "OldGroupName", "NewGroupName", "resource_group"

	pendingID, err := q.CreateContactReplacement(ctx, db.CreateContactReplacementParams{
		EntityKind: kind, EntityName: oldName, ContactType: "Administrative Contact", Rank: "Primary",
		RequesterUserID: requester, RequesterName: "Requester",
	})
	if err != nil {
		t.Fatalf("CreateContactReplacement (pending): %v", err)
	}
	decidedID, err := q.CreateContactReplacement(ctx, db.CreateContactReplacementParams{
		EntityKind: kind, EntityName: oldName, ContactType: "Security Contact", Rank: "Primary",
		RequesterUserID: requester, RequesterName: "Requester",
	})
	if err != nil {
		t.Fatalf("CreateContactReplacement (to-be-decided): %v", err)
	}
	if err := q.DecideContactReplacement(ctx, decidedID, "approved", requester); err != nil {
		t.Fatalf("DecideContactReplacement: %v", err)
	}

	claim := func(name string) []byte {
		b, err := json.Marshal(map[string]string{"entity_kind": kind, "entity_id": name})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if _, err := q.CreateInvite(ctx, db.CreateInviteParams{
		Kind: "role_claim", TokenHash: []byte("token-unused"),
		ClaimJSON: claim(oldName), ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("CreateInvite (unused): %v", err)
	}
	usedInviteID, err := q.CreateInvite(ctx, db.CreateInviteParams{
		Kind: "role_claim", TokenHash: []byte("token-used"),
		ClaimJSON: claim(oldName), ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateInvite (to-be-used): %v", err)
	}
	if err := q.MarkInviteUsed(ctx, usedInviteID, requester); err != nil {
		t.Fatalf("MarkInviteUsed: %v", err)
	}

	if err := q.RepointEntityNameReferences(ctx, kind, oldName, newName); err != nil {
		t.Fatalf("RepointEntityNameReferences: %v", err)
	}

	pending, err := q.GetContactReplacement(ctx, pendingID)
	if err != nil {
		t.Fatalf("GetContactReplacement (pending): %v", err)
	}
	if pending.EntityName != newName {
		t.Errorf("pending replacement's entity_name = %q, want %q (a live request must follow the rename)", pending.EntityName, newName)
	}

	decided, err := q.GetContactReplacement(ctx, decidedID)
	if err != nil {
		t.Fatalf("GetContactReplacement (decided): %v", err)
	}
	if decided.EntityName != oldName {
		t.Errorf("decided replacement's entity_name = %q, want unchanged %q (history must not be rewritten)", decided.EntityName, oldName)
	}

	unused, err := q.GetInviteByTokenHash(ctx, []byte("token-unused"))
	if err != nil {
		t.Fatalf("GetInviteByTokenHash (unused): %v", err)
	}
	var unusedClaim map[string]string
	if err := json.Unmarshal(unused.ClaimJSON, &unusedClaim); err != nil {
		t.Fatal(err)
	}
	if unusedClaim["entity_id"] != newName {
		t.Errorf("unused invite's claim.entity_id = %q, want %q", unusedClaim["entity_id"], newName)
	}

	used, err := q.GetInviteByTokenHash(ctx, []byte("token-used"))
	if err != nil {
		t.Fatalf("GetInviteByTokenHash (used): %v", err)
	}
	var usedClaim map[string]string
	if err := json.Unmarshal(used.ClaimJSON, &usedClaim); err != nil {
		t.Fatal(err)
	}
	if usedClaim["entity_id"] != oldName {
		t.Errorf("used invite's claim.entity_id = %q, want unchanged %q (a consumed invite is history)", usedClaim["entity_id"], oldName)
	}
}

// TestGetContactSlot_MissingSlotIsNotAnError guards a subtle behavior: a
// contact slot that doesn't exist (wrong entity, wrong rank, nothing set
// yet) must come back as "not found", never as a Go error -- callers rely
// on this to distinguish "nothing to hand off" from a real failure.
func TestGetContactSlot_MissingSlotIsNotAnError(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	slot, err := q.GetContactSlot(ctx, "resource_group", "does-not-exist", "Administrative Contact", "Primary")
	if err != nil {
		t.Fatalf("GetContactSlot returned an error for a missing slot: %v", err)
	}
	if slot.Found {
		t.Errorf("got Found=true for a slot that was never created: %+v", slot)
	}
}
