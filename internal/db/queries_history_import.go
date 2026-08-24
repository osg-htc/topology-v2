package db

import (
	"context"
	"encoding/json"
	"time"
)

// InsertHistoricalProposalParams is one already-applied historical change
// record replayed from a git commit -- see internal/githistory. Unlike
// CreateProposal, this writes an explicit created_at/updated_at (the
// commit's own author time, not NOW()), writes no change_proposal_revisions
// row (a replayed fact was never drafted or revised, so there's nothing to
// version), and is keyed for idempotent reruns via SourceCommitSHA.
type InsertHistoricalProposalParams struct {
	EntityKind      string
	TargetName      string
	Operation       string
	ProposedState   json.RawMessage
	BaseVersion     json.RawMessage
	SchemaVersion   int
	CreatedBy       string
	CommittedAt     time.Time
	SourceCommitSHA string
}

// InsertHistoricalProposal inserts one already-applied historical proposal.
func (q *Queries) InsertHistoricalProposal(ctx context.Context, p InsertHistoricalProposalParams) (string, error) {
	schemaVer := p.SchemaVersion
	if schemaVer == 0 {
		schemaVer = 1
	}
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO change_proposals
		   (entity_kind, target_name, operation, proposed_state, proposed_schema_version,
		    base_version, status, created_by, created_at, updated_at, source_commit_sha)
		 VALUES ($1,$2,$3,$4,$5,$6,'applied',$7,$8,$8,$9) RETURNING id`,
		p.EntityKind, nullString(p.TargetName), p.Operation, p.ProposedState, schemaVer,
		nullBytes(p.BaseVersion), p.CreatedBy, p.CommittedAt, p.SourceCommitSHA).Scan(&id)
	return id, err
}

// ListImportedCommitSHAs returns every source_commit_sha already recorded, so
// a re-run of the history importer can skip commits it already wrote instead
// of needing a separate checkpoint file.
func (q *Queries) ListImportedCommitSHAs(ctx context.Context) (map[string]bool, error) {
	rows, err := q.pool.Query(ctx, `SELECT source_commit_sha FROM change_proposals WHERE source_commit_sha IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var sha string
		if err := rows.Scan(&sha); err != nil {
			return nil, err
		}
		out[sha] = true
	}
	return out, rows.Err()
}

// UpsertHistoricalAuthor returns the id of a placeholder user representing
// one distinct historical git commit author, creating it if this is the
// first time that legacyKey has been seen. legacyKey is expected to be
// "git-import:<email>" (or a hashed fallback for an empty/malformed email) --
// see internal/githistory/authors.go -- so distinct historical identities
// dedupe via the same partial unique index users.legacy_contact_id already
// has (002_auth.sql), without a new column or constraint.
func (q *Queries) UpsertHistoricalAuthor(ctx context.Context, displayName, legacyKey string) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO users (display_name, legacy_contact_id)
		 VALUES ($1, $2)
		 ON CONFLICT (legacy_contact_id) WHERE legacy_contact_id IS NOT NULL
		 DO UPDATE SET legacy_contact_id = EXCLUDED.legacy_contact_id
		 RETURNING id`,
		displayName, legacyKey).Scan(&id)
	return id, err
}
