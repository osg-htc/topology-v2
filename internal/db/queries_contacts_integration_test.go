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
}
