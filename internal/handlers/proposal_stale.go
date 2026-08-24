package handlers

// TEMPORARY dev-only guard against a proposal silently overwriting a change
// made to the same entity while it was pending. It compares the entity's
// updated_at at the moment a proposal's base was snapshotted against its
// current updated_at at apply time; a mismatch means the live entity moved
// on since this proposal branched off, so applying it would blindly clobber
// whatever changed. This is a cheap approximation of real optimistic-
// concurrency control -- not a merge, just a refuse-if-stale check -- meant
// to be replaced by a proper design later.
//
// To remove: delete this file, then revert its call sites in proposals.go
// (grep entityUpdatedAt/BaseUpdatedAt/BaseStale/errStaleBase), the
// base_updated_at column (migration 014, `-- +goose Down`), and the
// corresponding fields in models.Proposal/CreateProposalParams/scanProposal.

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
)

var errStaleBase = errors.New("this proposal's base is out of date — the record has changed since this proposal was created; recreate it from the current state")

// entityUpdatedAt returns the live entity's current updated_at, or nil if
// the kind isn't snapshot-eligible (downtime, bundle) or the row is gone --
// soft-deleted or never existed. Both nil cases are treated as stale by
// callers, which also catches "the entity was deleted out from under this
// proposal" as a side effect, for free.
func entityUpdatedAt(ctx context.Context, q *db.Queries, entityKind, targetName string) (*time.Time, error) {
	if targetName == "" {
		return nil, nil
	}
	switch entityKind {
	case models.KindFacility:
		row, err := q.GetFacilityRow(ctx, targetName)
		if err != nil {
			return nil, nil
		}
		return &row.UpdatedAt, nil
	case models.KindSite:
		row, err := q.GetSiteRow(ctx, targetName)
		if err != nil {
			return nil, nil
		}
		return &row.UpdatedAt, nil
	case models.KindResourceGroup:
		row, err := q.GetResourceGroupRow(ctx, targetName)
		if err != nil {
			return nil, nil
		}
		return &row.UpdatedAt, nil
	case models.KindResource:
		topID, err := strconv.ParseInt(targetName, 10, 64)
		if err != nil {
			return nil, nil
		}
		row, err := q.GetResourceRow(ctx, topID)
		if err != nil {
			return nil, nil
		}
		return &row.UpdatedAt, nil
	case models.KindProject:
		row, err := q.GetProjectByName(ctx, targetName)
		if err != nil {
			return nil, nil
		}
		return &row.UpdatedAt, nil
	default:
		return nil, nil // downtime, bundle: not snapshot-eligible, nothing to compare
	}
}
