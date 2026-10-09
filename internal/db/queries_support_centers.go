package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SupportCenterFull is a full support-center row. Extra is the lossless
// catch-all for everything support-centers.yaml carries that has no column of
// its own -- chiefly the Contacts block (40 of 52 real centers) -- so an edit
// or a backup round trip never drops it.
type SupportCenterFull struct {
	ID          int64
	Name        string
	LongName    string
	Community   string
	Description string
	Extra       []byte
	// UpdatedAt feeds the proposal stale-base guard.
	UpdatedAt time.Time
}

const supportCenterCols = `id, name, COALESCE(long_name,''), COALESCE(community,''),
	COALESCE(description,''), extra, updated_at`

func scanSupportCenter(row interface{ Scan(...any) error }) (*SupportCenterFull, error) {
	var s SupportCenterFull
	if err := row.Scan(&s.ID, &s.Name, &s.LongName, &s.Community, &s.Description, &s.Extra, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

// UpsertSupportCenter inserts or updates a live support center by name
// (the importer's path). Extra may be nil.
func (q *Queries) UpsertSupportCenter(ctx context.Context, id int64, name, longName, community, description string) error {
	return q.UpsertSupportCenterFull(ctx, SupportCenterFull{
		ID: id, Name: name, LongName: longName, Community: community, Description: description,
	})
}

// UpsertSupportCenterFull is UpsertSupportCenter carrying Extra too.
func (q *Queries) UpsertSupportCenterFull(ctx context.Context, s SupportCenterFull) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO support_centers (id, name, long_name, community, description, extra)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (name) WHERE deleted_at IS NULL
		 DO UPDATE SET id=$1, long_name=$3, community=$4, description=$5, extra=$6, updated_at=NOW()`,
		s.ID, s.Name, nullString(s.LongName), nullString(s.Community), nullString(s.Description), nullBytes(s.Extra))
	return err
}

// ErrSupportCenterExists is returned by InsertSupportCenter for a name that is
// already live.
var ErrSupportCenterExists = errors.New("a support center with that name already exists")

// InsertSupportCenter creates a new support center. A previously soft-deleted
// center with the same id (ids derive from the name, so re-creating a deleted
// name lands on the same id) is revived in place rather than colliding on the
// primary key; a live one with the same name is an error.
func (q *Queries) InsertSupportCenter(ctx context.Context, s SupportCenterFull) error {
	var live bool
	if err := q.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM support_centers WHERE name=$1 AND deleted_at IS NULL)`, s.Name).Scan(&live); err != nil {
		return err
	}
	if live {
		return ErrSupportCenterExists
	}
	_, err := q.pool.Exec(ctx,
		`INSERT INTO support_centers (id, name, long_name, community, description, extra)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (id) DO UPDATE SET name=$2, long_name=$3, community=$4, description=$5,
		    extra=$6, deleted_at=NULL, deleted_by=NULL, updated_at=NOW()`,
		s.ID, s.Name, nullString(s.LongName), nullString(s.Community), nullString(s.Description), nullBytes(s.Extra))
	return err
}

// GetSupportCenter fetches one live support center by name.
func (q *Queries) GetSupportCenter(ctx context.Context, name string) (*SupportCenterFull, error) {
	return scanSupportCenter(q.pool.QueryRow(ctx,
		`SELECT `+supportCenterCols+` FROM support_centers WHERE name=$1 AND deleted_at IS NULL`, name))
}

// UpdateSupportCenterFields updates a live support center in place, keyed by
// its current name. The id is never touched: resource groups and the public
// feeds identify a center by it, and a name edit must not move it.
func (q *Queries) UpdateSupportCenterFields(ctx context.Context, targetName string, s SupportCenterFull) error {
	tag, err := q.pool.Exec(ctx,
		`UPDATE support_centers SET name=$2, long_name=$3, community=$4, description=$5, extra=$6, updated_at=NOW()
		 WHERE name=$1 AND deleted_at IS NULL`,
		targetName, s.Name, nullString(s.LongName), nullString(s.Community), nullString(s.Description), nullBytes(s.Extra))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("support center %q not found", targetName)
	}
	return nil
}

// SoftDeleteSupportCenterByName soft-deletes a support center.
func (q *Queries) SoftDeleteSupportCenterByName(ctx context.Context, name, byUser string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE support_centers SET deleted_at=NOW(), deleted_by=$2 WHERE name=$1 AND deleted_at IS NULL`,
		name, nullString(byUser))
	return err
}

// RGNamesUsingSupportCenter lists live resource groups whose SupportCenter is
// the given name (resource_groups.support_center holds the name, not an id).
func (q *Queries) RGNamesUsingSupportCenter(ctx context.Context, name string) ([]string, error) {
	return q.childNames(ctx,
		`SELECT name FROM resource_groups WHERE support_center=$1 AND deleted_at IS NULL ORDER BY name`, name)
}

// SupportCenterIDByName returns the id for a live support center name.
func (q *Queries) SupportCenterIDByName(ctx context.Context, name string) (int64, bool) {
	var id int64
	if err := q.pool.QueryRow(ctx,
		`SELECT id FROM support_centers WHERE name = $1 AND deleted_at IS NULL`, name).Scan(&id); err != nil {
		return 0, false
	}
	return id, true
}

// ListAllSupportCenters returns every live support center, ordered by name.
func (q *Queries) ListAllSupportCenters(ctx context.Context) ([]SupportCenterFull, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT `+supportCenterCols+` FROM support_centers WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SupportCenterFull
	for rows.Next() {
		s, err := scanSupportCenter(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// SupportCenterBrowseRow is one support center with how many live resource
// groups use it, for the list page.
type SupportCenterBrowseRow struct {
	Name           string `json:"name"`
	ID             int64  `json:"id"`
	LongName       string `json:"long_name"`
	Community      string `json:"community"`
	Description    string `json:"description"`
	ResourceGroups int    `json:"resource_group_count"`
}

// ListBrowseSupportCenters returns live support centers with usage counts.
func (q *Queries) ListBrowseSupportCenters(ctx context.Context) ([]SupportCenterBrowseRow, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT sc.name, sc.id, COALESCE(sc.long_name,''), COALESCE(sc.community,''), COALESCE(sc.description,''),
		        (SELECT COUNT(*) FROM resource_groups rg WHERE rg.support_center = sc.name AND rg.deleted_at IS NULL)
		 FROM support_centers sc WHERE sc.deleted_at IS NULL ORDER BY sc.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SupportCenterBrowseRow{}
	for rows.Next() {
		var r SupportCenterBrowseRow
		if err := rows.Scan(&r.Name, &r.ID, &r.LongName, &r.Community, &r.Description, &r.ResourceGroups); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RepointResourceGroupSupportCenter rewrites the support-center name on every
// live resource group that used oldName, after a support center is renamed.
func (q *Queries) RepointResourceGroupSupportCenter(ctx context.Context, oldName, newName string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE resource_groups SET support_center=$2, updated_at=NOW() WHERE support_center=$1 AND deleted_at IS NULL`,
		oldName, newName)
	return err
}

// SupportCenterNameByID returns the live support center name holding id, or "".
func (q *Queries) SupportCenterNameByID(ctx context.Context, id int64) (string, error) {
	var name string
	err := q.pool.QueryRow(ctx,
		`SELECT name FROM support_centers WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return name, err
}
