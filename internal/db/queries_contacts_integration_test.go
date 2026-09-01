// package db_test (external, not the internal db package): testsupport
// itself imports internal/db to build a *db.Queries, so a same-package test
// file here can't import testsupport without an import cycle.
package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/testsupport"
)

// TestReplaceEntityContacts covers the real logic in ReplaceEntityContacts
// (queries_contacts.go), not just that it inserts rows: rank is derived
// independently per contact type from list order, a blank entry (no name,
// no id) is skipped rather than inserted as an empty contact, and replacing
// the set actually retires the old rows (ListEntityContacts must not still
// return them).
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise.
func TestReplaceEntityContacts(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	const facName = "regtest-contacts-facility"
	if _, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000050, Name: facName, IDExplicit: true}); err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}

	t.Run("rank derived per type, blank entries skipped", func(t *testing.T) {
		contacts := []db.EntityContact{
			{ContactType: "Administrative Contact", Name: "Alice"},
			{ContactType: "Administrative Contact", Name: "Bob"},
			{ContactType: "Security Contact", Name: "Carol"},
			{ContactType: "Administrative Contact"}, // blank -- no name, no id
			{ContactType: "Administrative Contact", Name: "Dave"},
		}
		if err := q.ReplaceEntityContacts(ctx, models.KindFacility, facName, facName, contacts, ""); err != nil {
			t.Fatalf("ReplaceEntityContacts: %v", err)
		}
		got, err := q.ListEntityContacts(ctx, models.KindFacility, facName)
		if err != nil {
			t.Fatalf("ListEntityContacts: %v", err)
		}
		byTypeAndRank := map[string]string{} // "type/rank" -> name
		for _, c := range got {
			byTypeAndRank[c.ContactType+"/"+c.Rank] = c.Name
		}

		if len(got) != 4 {
			t.Fatalf("got %d contacts, want 4 (the blank entry must be skipped): %+v", len(got), got)
		}
		if byTypeAndRank["Administrative Contact/Primary"] != "Alice" {
			t.Errorf("Administrative/Primary = %q, want Alice", byTypeAndRank["Administrative Contact/Primary"])
		}
		if byTypeAndRank["Administrative Contact/Secondary"] != "Bob" {
			t.Errorf("Administrative/Secondary = %q, want Bob", byTypeAndRank["Administrative Contact/Secondary"])
		}
		// Dave is Administrative's 3rd non-blank entry (the blank one was
		// skipped and must not have consumed a rank slot).
		if byTypeAndRank["Administrative Contact/Tertiary"] != "Dave" {
			t.Errorf("Administrative/Tertiary = %q, want Dave (rank must skip the blank entry, not leave a gap)", byTypeAndRank["Administrative Contact/Tertiary"])
		}
		// Security's rank sequence is independent of Administrative's.
		if byTypeAndRank["Security Contact/Primary"] != "Carol" {
			t.Errorf("Security/Primary = %q, want Carol (rank must be independent per contact type)", byTypeAndRank["Security Contact/Primary"])
		}
	})

	t.Run("replacing the set retires the old rows", func(t *testing.T) {
		if err := q.ReplaceEntityContacts(ctx, models.KindFacility, facName, facName, []db.EntityContact{
			{ContactType: "Administrative Contact", Name: "Eve"},
		}, ""); err != nil {
			t.Fatalf("ReplaceEntityContacts: %v", err)
		}
		got, err := q.ListEntityContacts(ctx, models.KindFacility, facName)
		if err != nil {
			t.Fatalf("ListEntityContacts: %v", err)
		}
		if len(got) != 1 || got[0].Name != "Eve" {
			t.Fatalf("got %+v, want exactly [Eve] -- the previous set (Alice/Bob/Dave/Carol) must be retired, not left alongside the new one", got)
		}
	})

	t.Run("the ID vivifies a provisioned user, keyed on that same ID", func(t *testing.T) {
		const legacyID = "legacy-only-id"
		if err := q.ReplaceEntityContacts(ctx, models.KindFacility, facName, facName, []db.EntityContact{
			{ContactType: "Administrative Contact", Name: "Legacy Contact", ID: legacyID},
		}, ""); err != nil {
			t.Fatalf("ReplaceEntityContacts: %v", err)
		}
		got, err := q.ListEntityContacts(ctx, models.KindFacility, facName)
		if err != nil {
			t.Fatalf("ListEntityContacts: %v", err)
		}
		if len(got) != 1 || got[0].ID != legacyID {
			t.Fatalf("got %+v, want [Legacy Contact/%s]", got, legacyID)
		}
		// The apply-time contact check (LegacyContactIDExists) must see this
		// same ID as resolved, since it's the one identifier a contact has.
		if exists, err := q.LegacyContactIDExists(ctx, legacyID); err != nil {
			t.Fatalf("LegacyContactIDExists: %v", err)
		} else if !exists {
			t.Fatalf("LegacyContactIDExists(%q) = false, want true -- writing a contact must vivify a user with this legacy_contact_id", legacyID)
		}
	})
}

// TestBackfillEntityContactUsers guards BackfillEntityContactUsers'
// idempotent two-step shape (mirroring the existing resource_contacts
// backfill): it links pre-existing legacy rows to a provisioned user without
// disturbing rows that already have one, and is safe to run twice.
func TestBackfillEntityContactUsers(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	const facName = "regtest-backfill-facility"
	if _, err := q.InsertFacility(ctx, db.FacilityRow{TopologyID: 900000060, Name: facName, IDExplicit: true}); err != nil {
		t.Fatalf("InsertFacility: %v", err)
	}
	// A legacy row with no linked user -- as if imported before this
	// requirement existed. ReplaceEntityContacts' own fallback would
	// normally vivify this at write time; insert it directly here to
	// simulate genuinely pre-existing data that predates any write through
	// that path.
	if err := q.AddEntityContact(ctx, models.KindFacility, facName, "Administrative Contact", "Primary", "Legacy Person", "legacy-backfill-id", ""); err != nil {
		t.Fatalf("AddEntityContact: %v", err)
	}

	linked, err := q.BackfillEntityContactUsers(ctx)
	if err != nil {
		t.Fatalf("BackfillEntityContactUsers: %v", err)
	}
	if linked < 1 {
		t.Fatalf("BackfillEntityContactUsers linked %d rows, want at least 1", linked)
	}
	if exists, err := q.LegacyContactIDExists(ctx, "legacy-backfill-id"); err != nil {
		t.Fatalf("LegacyContactIDExists: %v", err)
	} else if !exists {
		t.Fatalf("LegacyContactIDExists(legacy-backfill-id) = false, want true -- the backfill must link the legacy row to a provisioned user")
	}

	// Idempotent: running it again must not error or double-link.
	if _, err := q.BackfillEntityContactUsers(ctx); err != nil {
		t.Fatalf("BackfillEntityContactUsers (second run): %v", err)
	}
}

// TestLegacyContactIDExists guards the sole check requireResolvedContacts/
// requireResolvedResourceContacts rely on: a garbage string must report
// false (not error), a real legacy_contact_id must report true, and an
// empty id must never match a row whose legacy_contact_id is NULL.
func TestLegacyContactIDExists(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run this test")
	}
	ctx := context.Background()
	_, q := testsupport.SetupSchema(t, dbURL)

	if ok, err := q.LegacyContactIDExists(ctx, "no-such-legacy-id"); err != nil {
		t.Fatalf("LegacyContactIDExists(garbage): %v", err)
	} else if ok {
		t.Fatalf("LegacyContactIDExists(garbage) = true, want false")
	}

	const legacyID = "regtest-real-legacy-id"
	if _, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "Real Person", Status: "active", LegacyContactID: legacyID}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if ok, err := q.LegacyContactIDExists(ctx, legacyID); err != nil {
		t.Fatalf("LegacyContactIDExists(real): %v", err)
	} else if !ok {
		t.Fatalf("LegacyContactIDExists(real) = false, want true")
	}

	// A user with no legacy_contact_id at all (NULL) must never match an
	// empty-string lookup.
	if _, err := q.CreateUser(ctx, db.CreateUserParams{DisplayName: "No Legacy ID", Status: "active"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if ok, err := q.LegacyContactIDExists(ctx, ""); err != nil {
		t.Fatalf("LegacyContactIDExists(\"\"): %v", err)
	} else if ok {
		t.Fatalf("LegacyContactIDExists(\"\") = true, want false -- must not match rows with a NULL legacy_contact_id")
	}
}
