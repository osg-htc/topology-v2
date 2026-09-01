package db

import "context"

// ranksByOrder maps a contact's 0-based position within its type to a rank.
// The UI only exposes ordering (reorder arrows); rank is derived here so the
// backend is the single source of truth for Primary/Secondary/Tertiary.
var ranksByOrder = []string{"Primary", "Secondary", "Tertiary"}

func rankForOrder(n int) string {
	if n >= len(ranksByOrder) {
		return ranksByOrder[len(ranksByOrder)-1]
	}
	return ranksByOrder[n]
}

// EntityContact is a contact on a resource group / site / facility. Rank is
// derived from list order at apply time (see ReplaceEntityContacts); callers
// supply contacts already in the desired order and may leave Rank empty.
// ID is the one identifier a contact has: the same v1 scheme (SHA1 of a
// lowercased email, or an OSG-prefixed CILogon id -- see emailSHA1 in
// internal/handlers/auth.go). The proposal-apply path requires it to match a
// real users.legacy_contact_id (see requireResolvedContacts in
// internal/handlers/proposals.go) -- there is no separate v2-native id.
type EntityContact struct {
	ContactType string `json:"contact_type"`
	Rank        string `json:"rank"`
	Name        string `json:"name"`
	ID          string `json:"id"`
}

// ReplaceEntityContacts sets the contacts for a parent entity (resource_group /
// site / facility): soft-deletes the current set and inserts the new one,
// bootstrapping a provisioned user per contact (a no-op find, not a create,
// for the ordinary proposal path -- ID has already been validated to match a
// real users.legacy_contact_id by the time this runs; see
// requireResolvedContacts).
func (q *Queries) ReplaceEntityContacts(ctx context.Context, kind, targetName, newName string, contacts []EntityContact, byUser string) error {
	if _, err := q.pool.Exec(ctx,
		`UPDATE entity_contacts SET deleted_at = NOW(), deleted_by = $3
		 WHERE entity_kind = $1 AND entity_name = $2 AND deleted_at IS NULL`,
		kind, targetName, nullString(byUser)); err != nil {
		return err
	}
	// Derive rank from order within each contact type (1st = Primary, …).
	perType := map[string]int{}
	for _, c := range contacts {
		if c.Name == "" && c.ID == "" {
			continue
		}
		rank := rankForOrder(perType[c.ContactType])
		perType[c.ContactType]++
		userID, _ := q.UpsertProvisionedContactUser(ctx, c.Name, c.ID)
		if _, err := q.pool.Exec(ctx,
			`INSERT INTO entity_contacts (entity_kind, entity_name, contact_type, rank, contact_name, contact_id, user_id)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			kind, newName, c.ContactType, rank, nullString(c.Name), nullString(c.ID), nullString(userID)); err != nil {
			return err
		}
	}
	return nil
}

// AddEntityContact appends a single contact to a parent entity (resource_group /
// site / facility), linking it to the given user. Used when a role_claim invite
// is accepted for one of these entities.
func (q *Queries) AddEntityContact(ctx context.Context, kind, name, contactType, rank, contactName, contactID, userID string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO entity_contacts (entity_kind, entity_name, contact_type, rank, contact_name, contact_id, user_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		kind, name, contactType, rank, nullString(contactName), nullString(contactID), nullString(userID))
	return err
}

// ListEntityContacts returns the active contacts for a parent entity.
func (q *Queries) ListEntityContacts(ctx context.Context, kind, name string) ([]EntityContact, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT contact_type, rank, COALESCE(contact_name,''), COALESCE(contact_id,'')
		 FROM entity_contacts
		 WHERE entity_kind = $1 AND entity_name = $2 AND deleted_at IS NULL
		 ORDER BY contact_type, rank`, kind, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]EntityContact, 0)
	for rows.Next() {
		var c EntityContact
		if err := rows.Scan(&c.ContactType, &c.Rank, &c.Name, &c.ID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
