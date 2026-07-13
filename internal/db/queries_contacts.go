package db

import "context"

// EntityContact is a contact on a resource group / site / facility.
type EntityContact struct {
	ContactType string `json:"contact_type"`
	Rank        string `json:"rank"`
	Name        string `json:"name"`
	ID          string `json:"id"`
}

// ReplaceEntityContacts sets the contacts for a parent entity (resource_group /
// site / facility): soft-deletes the current set and inserts the new one,
// bootstrapping a provisioned user per contact.
func (q *Queries) ReplaceEntityContacts(ctx context.Context, kind, name string, contacts []EntityContact, byUser string) error {
	if _, err := q.pool.Exec(ctx,
		`UPDATE entity_contacts SET deleted_at = NOW(), deleted_by = $3
		 WHERE entity_kind = $1 AND entity_name = $2 AND deleted_at IS NULL`,
		kind, name, nullString(byUser)); err != nil {
		return err
	}
	for _, c := range contacts {
		if c.Name == "" && c.ID == "" {
			continue
		}
		userID, _ := q.UpsertProvisionedContactUser(ctx, c.Name, c.ID)
		if _, err := q.pool.Exec(ctx,
			`INSERT INTO entity_contacts (entity_kind, entity_name, contact_type, rank, contact_name, contact_id, user_id)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			kind, name, c.ContactType, c.Rank, nullString(c.Name), nullString(c.ID), nullString(userID)); err != nil {
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
