package handlers

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/testsupport"
	"github.com/bbockelm/topology-v2/internal/topology"
)

// TestApplyProposal_ThreeWayMerge exercises applyProposal's lock+merge step
// (proposal_merge3.go) against a real database: it's the replacement for
// the old, now-removed stale-base guard, so this covers the two behaviors
// that guard could never tell apart -- a genuinely unrelated concurrent
// change (which must compose automatically) and a real same-field conflict
// (which must block the apply and leave the live row untouched).
//
// Requires Postgres reachable at TOPOLOGY_TEST_DATABASE_URL; skipped
// otherwise, so `go test ./...` stays green without a database.
func TestApplyProposal_ThreeWayMerge(t *testing.T) {
	dbURL := os.Getenv("TOPOLOGY_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("set TOPOLOGY_TEST_DATABASE_URL to run the three-way-merge integration test")
	}

	ctx := context.Background()
	pool, q := testsupport.SetupSchema(t, dbURL)
	h := &Handler{queries: q}

	const (
		topID     int64 = 900000101
		rtName          = "regtest-merge-resource"
		groupName       = "regtest-merge-group"
	)

	var actorID, target string
	t.Run("seed", func(t *testing.T) {
		var err error
		actorID, err = q.CreateUser(ctx, db.CreateUserParams{
			DisplayName: "regtest-merge-actor", Status: "active", IsProvisioned: true,
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		facID, err := q.InsertFacility(ctx, db.FacilityRow{
			TopologyID: 900000110, Name: "regtest-merge-facility", IDExplicit: true,
		})
		if err != nil {
			t.Fatalf("InsertFacility: %v", err)
		}
		siteID, err := q.InsertSite(ctx, db.SiteRow{
			TopologyID: 900000111, FacilityID: facID, Name: "regtest-merge-site", IDExplicit: true,
		})
		if err != nil {
			t.Fatalf("InsertSite: %v", err)
		}
		rgID, err := q.InsertResourceGroup(ctx, db.ResourceGroupRow{
			GroupID: 900000112, SiteID: siteID, Name: groupName, IDExplicit: true,
		})
		if err != nil {
			t.Fatalf("InsertResourceGroup: %v", err)
		}
		active := true
		if err := q.InsertResource(ctx, db.ResourceRow{
			TopologyID: topID, ResourceGroupID: rgID, Name: rtName,
			Active: &active, FQDN: "merge-before.example.org", Description: "before",
			IDExplicit: true,
		}); err != nil {
			t.Fatalf("InsertResource: %v", err)
		}
		target = strconv.FormatInt(topID, 10)
	})

	applyDirect := func(t *testing.T, fqdn, description string) {
		active := true
		rp := resourceProposal{
			ResourceGroup: groupName, Name: rtName,
			Resource: topology.Resource{Active: &active, FQDN: fqdn, Description: description},
		}
		state, err := json.Marshal(rp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		p := &models.Proposal{
			EntityKind: models.KindResource, Operation: models.OpUpdate,
			TargetName: target, ProposedState: state,
		}
		if err := h.applyResourceProposal(ctx, q, p, actorID); err != nil {
			t.Fatalf("applyResourceProposal (direct): %v", err)
		}
	}

	t.Run("non-conflicting concurrent edits compose", func(t *testing.T) {
		base := snapshotEntity(ctx, q, models.KindResource, target)
		if base == nil {
			t.Fatalf("snapshotEntity returned nil base")
		}

		// Someone else's proposal lands first, changing an unrelated field.
		applyDirect(t, "merge-before.example.org", "changed-by-someone-else")

		// Our own proposal, branched from the pre-that-change base, changes a
		// different field.
		active := true
		rp := resourceProposal{
			ResourceGroup: groupName, Name: rtName,
			Resource: topology.Resource{
				Active: &active, FQDN: "merge-after.example.org", Description: "before",
				ContactLists: map[string]map[string]topology.Contact{},
			},
		}
		state, err := json.Marshal(rp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		p := &models.Proposal{
			EntityKind: models.KindResource, Operation: models.OpUpdate,
			TargetName: target, ProposedState: state, BaseVersion: base,
		}

		if err := q.WithTx(ctx, func(tx *db.Queries) error {
			return h.applyProposal(ctx, tx, p, actorID)
		}); err != nil {
			t.Fatalf("applyProposal: expected non-conflicting merge to succeed, got: %v", err)
		}

		got, err := q.GetResourceRow(ctx, topID)
		if err != nil {
			t.Fatalf("GetResourceRow: %v", err)
		}
		if got.FQDN != "merge-after.example.org" {
			t.Fatalf("FQDN = %q, want our own change to have applied", got.FQDN)
		}
		if got.Description != "changed-by-someone-else" {
			t.Fatalf("Description = %q, want the independent concurrent change preserved", got.Description)
		}
	})

	t.Run("same-field conflict is rejected, live row untouched", func(t *testing.T) {
		base := snapshotEntity(ctx, q, models.KindResource, target)
		if base == nil {
			t.Fatalf("snapshotEntity returned nil base")
		}

		// Someone else's proposal lands first, changing the SAME field we're
		// about to propose changing.
		applyDirect(t, "merge-conflict-theirs.example.org", "changed-by-someone-else")

		active := true
		rp := resourceProposal{
			ResourceGroup: groupName, Name: rtName,
			Resource: topology.Resource{Active: &active, FQDN: "merge-conflict-ours.example.org", Description: "changed-by-someone-else"},
		}
		state, err := json.Marshal(rp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		p := &models.Proposal{
			EntityKind: models.KindResource, Operation: models.OpUpdate,
			TargetName: target, ProposedState: state, BaseVersion: base,
		}

		err = q.WithTx(ctx, func(tx *db.Queries) error {
			return h.applyProposal(ctx, tx, p, actorID)
		})
		if err == nil {
			t.Fatalf("applyProposal: expected a conflict error, got success")
		}
		if !strings.Contains(err.Error(), "resource.FQDN") {
			t.Fatalf("conflict error = %q, want it to name resource.FQDN", err.Error())
		}

		got, err := q.GetResourceRow(ctx, topID)
		if err != nil {
			t.Fatalf("GetResourceRow: %v", err)
		}
		if got.FQDN != "merge-conflict-theirs.example.org" {
			t.Fatalf("FQDN = %q, want the rejected apply to have left the live row untouched", got.FQDN)
		}
	})

	t.Run("lock_timeout fires under a held conflicting lock", func(t *testing.T) {
		base := snapshotEntity(ctx, q, models.KindResource, target)
		if base == nil {
			t.Fatalf("snapshotEntity returned nil base")
		}

		// Hold a conflicting row lock on a separate connection, simulating a
		// concurrent apply that's mid-transaction -- lockEntityRow's
		// SetLockTimeout must make our own apply fail fast rather than hang.
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire raw connection: %v", err)
		}
		defer conn.Release()
		holder, err := conn.Begin(ctx)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer holder.Rollback(ctx)
		if _, err := holder.Exec(ctx, `SELECT 1 FROM resources WHERE topology_id = $1 FOR UPDATE`, topID); err != nil {
			t.Fatalf("hold lock: %v", err)
		}

		active := true
		rp := resourceProposal{
			ResourceGroup: groupName, Name: rtName,
			Resource: topology.Resource{
				Active: &active, FQDN: "merge-lock-timeout.example.org", Description: "changed-by-someone-else",
				ContactLists: map[string]map[string]topology.Contact{},
			},
		}
		state, err := json.Marshal(rp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		p := &models.Proposal{
			EntityKind: models.KindResource, Operation: models.OpUpdate,
			TargetName: target, ProposedState: state, BaseVersion: base,
		}

		start := time.Now()
		applyErr := q.WithTx(ctx, func(tx *db.Queries) error {
			return h.applyProposal(ctx, tx, p, actorID)
		})
		elapsed := time.Since(start)
		if err := holder.Rollback(ctx); err != nil {
			t.Fatalf("release held lock: %v", err)
		}

		if applyErr == nil {
			t.Fatalf("applyProposal: expected a lock_timeout error while the row was held, got success")
		}
		if elapsed > 10*time.Second {
			t.Fatalf("applyProposal took %v to fail, want it to fail fast via lock_timeout (~5s)", elapsed)
		}
		t.Logf("applyProposal failed after %v: %v", elapsed, applyErr)
	})
}
