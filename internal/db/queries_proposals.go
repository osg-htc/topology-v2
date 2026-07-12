package db

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/bbockelm/topology-v2/internal/models"
)

// ---- proposals ----

// CreateProposalParams holds fields for a new proposal.
type CreateProposalParams struct {
	EntityKind    string
	TargetName    string
	Operation     string
	ProposedState json.RawMessage
	BaseVersion   json.RawMessage
	Status        string
	SchemaVersion int
	CreatedBy     string
	Note          string
}

// CreateProposal inserts a proposal and its first revision atomically.
func (q *Queries) CreateProposal(ctx context.Context, p CreateProposalParams) (string, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	state := p.ProposedState
	if len(state) == 0 {
		state = json.RawMessage(`{}`)
	}
	schemaVer := p.SchemaVersion
	if schemaVer == 0 {
		schemaVer = 1
	}
	var id string
	err = tx.QueryRow(ctx,
		`INSERT INTO change_proposals
		   (entity_kind, target_name, operation, proposed_state, proposed_schema_version,
		    base_version, status, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		p.EntityKind, nullString(p.TargetName), p.Operation, state, schemaVer,
		nullBytes(p.BaseVersion), p.Status, p.CreatedBy).Scan(&id)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO change_proposal_revisions (proposal_id, revision_no, proposed_state, edited_by, note)
		 VALUES ($1, 1, $2, $3, $4)`, id, state, p.CreatedBy, nullString(p.Note)); err != nil {
		return "", err
	}
	return id, tx.Commit(ctx)
}

// AddRevision appends a new revision and updates the proposal head.
func (q *Queries) AddRevision(ctx context.Context, proposalID string, state json.RawMessage, editedBy, note string) error {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var next int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(revision_no),0)+1 FROM change_proposal_revisions WHERE proposal_id = $1`,
		proposalID).Scan(&next); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO change_proposal_revisions (proposal_id, revision_no, proposed_state, edited_by, note)
		 VALUES ($1,$2,$3,$4,$5)`, proposalID, next, state, editedBy, nullString(note)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE change_proposals SET proposed_state = $2, updated_at = NOW() WHERE id = $1`,
		proposalID, state); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UpdateProposalStatus sets status, reviewer, and review note.
func (q *Queries) UpdateProposalStatus(ctx context.Context, id, status, reviewer, note string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE change_proposals
		 SET status = $2, assigned_reviewer = $3, review_note = $4, updated_at = NOW()
		 WHERE id = $1`, id, status, nullString(reviewer), nullString(note))
	return err
}

func scanProposal(row pgx.Row) (*models.Proposal, error) {
	p := &models.Proposal{}
	var target, reviewer, note *string
	var base []byte
	err := row.Scan(&p.ID, &p.EntityKind, &target, &p.Operation, &p.ProposedState,
		&p.SchemaVersion, &base, &p.Status, &p.CreatedBy, &reviewer, &note, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.TargetName = deref(target)
	p.AssignedReviewer = deref(reviewer)
	p.ReviewNote = deref(note)
	if len(base) > 0 {
		p.BaseVersion = base
	}
	return p, nil
}

const proposalCols = `id, entity_kind, target_name, operation, proposed_state,
	proposed_schema_version, base_version, status, created_by, assigned_reviewer,
	review_note, created_at, updated_at`

// GetProposal fetches one proposal (without revisions).
func (q *Queries) GetProposal(ctx context.Context, id string) (*models.Proposal, error) {
	return scanProposal(q.pool.QueryRow(ctx,
		`SELECT `+proposalCols+` FROM change_proposals WHERE id = $1`, id))
}

// ListProposalsByCreator returns a user's proposals, newest first.
func (q *Queries) ListProposalsByCreator(ctx context.Context, userID string) ([]*models.Proposal, error) {
	return q.listProposals(ctx,
		`SELECT `+proposalCols+` FROM change_proposals WHERE created_by = $1 ORDER BY updated_at DESC`, userID)
}

// ListPendingProposals returns all pending proposals (reviewer queue).
func (q *Queries) ListPendingProposals(ctx context.Context) ([]*models.Proposal, error) {
	return q.listProposals(ctx,
		`SELECT `+proposalCols+` FROM change_proposals WHERE status = 'pending' ORDER BY updated_at ASC`)
}

func (q *Queries) listProposals(ctx context.Context, sql string, args ...any) ([]*models.Proposal, error) {
	rows, err := q.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Proposal
	for rows.Next() {
		p, err := scanProposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListRevisions returns a proposal's edit history, oldest first.
func (q *Queries) ListRevisions(ctx context.Context, proposalID string) ([]models.ProposalRevision, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT revision_no, proposed_state, edited_by, COALESCE(note,''), created_at
		 FROM change_proposal_revisions WHERE proposal_id = $1 ORDER BY revision_no`, proposalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ProposalRevision
	for rows.Next() {
		var r models.ProposalRevision
		if err := rows.Scan(&r.RevisionNo, &r.ProposedState, &r.EditedBy, &r.Note, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---- audit log (append-only) ----

// AppendAudit inserts an immutable audit-log row.
func (q *Queries) AppendAudit(ctx context.Context, actor, action, entityKind, entityID, proposalID string, detail json.RawMessage) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO audit_log (actor_user_id, action, entity_kind, entity_id, proposal_id, detail)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		nullString(actor), action, nullString(entityKind), nullString(entityID),
		nullString(proposalID), nullBytes(detail))
	return err
}

// ListAudit returns recent audit entries (most recent first).
func (q *Queries) ListAudit(ctx context.Context, limit int) ([]models.AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := q.pool.Query(ctx,
		`SELECT id, COALESCE(actor_user_id::text,''), action, COALESCE(entity_kind,''),
		        COALESCE(entity_id,''), COALESCE(proposal_id::text,''), detail, created_at
		 FROM audit_log ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.AuditEntry
	for rows.Next() {
		var e models.AuditEntry
		if err := rows.Scan(&e.ID, &e.ActorUserID, &e.Action, &e.EntityKind,
			&e.EntityID, &e.ProposalID, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---- helpers used when applying approved proposals ----

// ResourceGroupIDByName returns the active resource group id for a name.
func (q *Queries) ResourceGroupIDByName(ctx context.Context, name string) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`SELECT id FROM resource_groups WHERE name = $1 AND deleted_at IS NULL`, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

// ResourceIDByName returns the active resource id for a name.
func (q *Queries) ResourceIDByName(ctx context.Context, name string) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`SELECT id FROM resources WHERE name = $1 AND deleted_at IS NULL`, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

// SoftDeleteResource marks a resource (and is a no-op if already gone).
func (q *Queries) SoftDeleteResource(ctx context.Context, id, byUser string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE resources SET deleted_at = NOW(), deleted_by = $2 WHERE id = $1 AND deleted_at IS NULL`,
		id, nullString(byUser))
	return err
}

// AddResourceContact assigns a contact/responsibility to a resource, linking a
// user when known (used by role-claim invite acceptance).
func (q *Queries) AddResourceContact(ctx context.Context, resourceID, contactType, rank, name, contactID, userID string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO resource_contacts (resource_id, contact_type, rank, contact_name, contact_id, user_id)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		resourceID, contactType, rank, nullString(name), nullString(contactID), nullString(userID))
	return err
}

// ResourcesForContactIDs returns active resources where the user is a contact,
// matched either by resolved user_id or by any of their legacy contact ids.
func (q *Queries) ResourcesForContactIDs(ctx context.Context, userID string, legacyIDs []string) ([]ResourceRow, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT DISTINCT r.id, r.topology_id, rg.name, r.name, r.active,
		        COALESCE(r.description,''), r.fqdn, COALESCE(r.dn,'')
		 FROM resources r
		 JOIN resource_groups rg ON rg.id = r.resource_group_id
		 JOIN resource_contacts rc ON rc.resource_id = r.id AND rc.deleted_at IS NULL
		 WHERE r.deleted_at IS NULL
		   AND (rc.user_id = $1 OR rc.contact_id = ANY($2))
		 ORDER BY r.name`, userID, legacyIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResourceRow
	for rows.Next() {
		var r ResourceRow
		if err := rows.Scan(&r.ID, &r.TopologyID, &r.RGName, &r.Name, &r.Active,
			&r.Description, &r.FQDN, &r.DN); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
