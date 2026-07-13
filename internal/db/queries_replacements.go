package db

import "context"

// ContactSlot is the current holder of a contact position on some entity.
type ContactSlot struct {
	Name   string
	UserID string // "" if the row has no linked user
	Found  bool
}

// GetContactSlot returns the current holder of a contact slot, identified by
// (entity_kind, entity_name, contact_type, rank). Resource slots live in
// resource_contacts; resource_group/site/facility slots in entity_contacts.
func (q *Queries) GetContactSlot(ctx context.Context, kind, name, contactType, rank string) (ContactSlot, error) {
	var s ContactSlot
	var uid *string
	var err error
	if kind == "resource" {
		err = q.pool.QueryRow(ctx,
			`SELECT COALESCE(rc.contact_name,''), rc.user_id
			 FROM resource_contacts rc JOIN resources r ON r.id = rc.resource_id
			 WHERE r.name = $1 AND r.deleted_at IS NULL AND rc.deleted_at IS NULL
			   AND rc.contact_type = $2 AND rc.rank = $3
			 LIMIT 1`, name, contactType, rank).Scan(&s.Name, &uid)
	} else {
		err = q.pool.QueryRow(ctx,
			`SELECT COALESCE(contact_name,''), user_id
			 FROM entity_contacts
			 WHERE entity_kind = $1 AND entity_name = $2 AND contact_type = $3 AND rank = $4
			   AND deleted_at IS NULL
			 LIMIT 1`, kind, name, contactType, rank).Scan(&s.Name, &uid)
	}
	if err != nil {
		return ContactSlot{}, nil // treat a missing slot as "not found", not an error
	}
	s.Found = true
	if uid != nil {
		s.UserID = *uid
	}
	return s, nil
}

// ReplaceContactSlot repoints a contact slot to a new holder: it soft-deletes
// the current row (if any) and inserts a fresh one linked to the new user.
func (q *Queries) ReplaceContactSlot(ctx context.Context, kind, name, contactType, rank, newName, newContactID, newUserID, byUser string) error {
	if kind == "resource" {
		var resID string
		if err := q.pool.QueryRow(ctx,
			`SELECT id FROM resources WHERE name = $1 AND deleted_at IS NULL`, name).Scan(&resID); err != nil {
			return ErrNotFound
		}
		if _, err := q.pool.Exec(ctx,
			`UPDATE resource_contacts SET deleted_at = NOW(), deleted_by = $4
			 WHERE resource_id = $1 AND contact_type = $2 AND rank = $3 AND deleted_at IS NULL`,
			resID, contactType, rank, nullString(byUser)); err != nil {
			return err
		}
		return q.AddResourceContact(ctx, resID, contactType, rank, newName, newContactID, newUserID)
	}
	if _, err := q.pool.Exec(ctx,
		`UPDATE entity_contacts SET deleted_at = NOW(), deleted_by = $5
		 WHERE entity_kind = $1 AND entity_name = $2 AND contact_type = $3 AND rank = $4
		   AND deleted_at IS NULL`,
		kind, name, contactType, rank, nullString(byUser)); err != nil {
		return err
	}
	return q.AddEntityContact(ctx, kind, name, contactType, rank, newName, newContactID, newUserID)
}

// ContactReplacement is a pending/decided request to take over a contact slot.
type ContactReplacement struct {
	ID            string `json:"id"`
	EntityKind    string `json:"entity_kind"`
	EntityName    string `json:"entity_name"`
	ContactType   string `json:"contact_type"`
	Rank          string `json:"rank"`
	IncumbentName string `json:"incumbent_name"`
	IncumbentUser string `json:"incumbent_user_id,omitempty"`
	RequesterUser string `json:"requester_user_id"`
	RequesterName string `json:"requester_name"`
	Status        string `json:"status"`
	Note          string `json:"note,omitempty"`
}

// CreateContactReplacementParams carries the fields for a new request.
type CreateContactReplacementParams struct {
	EntityKind, EntityName, ContactType, Rank string
	IncumbentUserID, IncumbentName            string
	RequesterUserID, RequesterName            string
	RequesterContactID, Note                  string
}

// CreateContactReplacement inserts a pending replacement request.
func (q *Queries) CreateContactReplacement(ctx context.Context, p CreateContactReplacementParams) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO contact_replacements
		   (entity_kind, entity_name, contact_type, rank, incumbent_user_id, incumbent_name,
		    requester_user_id, requester_name, requester_contact_id, note)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		p.EntityKind, p.EntityName, p.ContactType, p.Rank,
		nullString(p.IncumbentUserID), nullString(p.IncumbentName),
		p.RequesterUserID, nullString(p.RequesterName), nullString(p.RequesterContactID),
		nullString(p.Note)).Scan(&id)
	return id, err
}

const replacementCols = `id, entity_kind, entity_name, contact_type, rank,
	COALESCE(incumbent_name,''), COALESCE(incumbent_user_id::text,''),
	requester_user_id::text, COALESCE(requester_name,''), status, COALESCE(note,'')`

func scanReplacement(rows interface{ Scan(...any) error }) (ContactReplacement, error) {
	var r ContactReplacement
	err := rows.Scan(&r.ID, &r.EntityKind, &r.EntityName, &r.ContactType, &r.Rank,
		&r.IncumbentName, &r.IncumbentUser, &r.RequesterUser, &r.RequesterName, &r.Status, &r.Note)
	return r, err
}

// GetContactReplacement fetches one request by id.
func (q *Queries) GetContactReplacement(ctx context.Context, id string) (ContactReplacement, error) {
	row := q.pool.QueryRow(ctx, `SELECT `+replacementCols+` FROM contact_replacements WHERE id = $1`, id)
	return scanReplacement(row)
}

// ListReplacementsForIncumbent lists pending requests to replace the given user.
func (q *Queries) ListReplacementsForIncumbent(ctx context.Context, userID string) ([]ContactReplacement, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT `+replacementCols+` FROM contact_replacements
		 WHERE incumbent_user_id = $1 AND status = 'pending' ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ContactReplacement, 0)
	for rows.Next() {
		r, err := scanReplacement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListReplacementsByRequester lists requests the given user has made.
func (q *Queries) ListReplacementsByRequester(ctx context.Context, userID string) ([]ContactReplacement, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT `+replacementCols+` FROM contact_replacements
		 WHERE requester_user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ContactReplacement, 0)
	for rows.Next() {
		r, err := scanReplacement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DecideContactReplacement records the outcome of a request.
func (q *Queries) DecideContactReplacement(ctx context.Context, id, status, decidedBy string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE contact_replacements SET status = $2, decided_at = NOW(), decided_by = $3
		 WHERE id = $1 AND status = 'pending'`, id, status, nullString(decidedBy))
	return err
}
