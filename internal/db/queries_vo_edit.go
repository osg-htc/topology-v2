package db

import (
	"context"
	"errors"
	"fmt"
)

// ErrVOExists is returned by InsertVO for a name that is already live.
var ErrVOExists = errors.New("a VO with that name already exists")

// GetVO fetches one live VO by name, including updated_at for the stale-base guard.
func (q *Queries) GetVO(ctx context.Context, name string) (*VORow, error) {
	var r VORow
	var raw string
	if err := q.pool.QueryRow(ctx,
		`SELECT name, vo_id, disable, raw_yaml, updated_at FROM vos WHERE name=$1 AND deleted_at IS NULL`, name,
	).Scan(&r.Name, &r.VOID, &r.Disable, &raw, &r.UpdatedAt); err != nil {
		return nil, err
	}
	r.Raw = []byte(raw)
	return &r, nil
}

// InsertVO creates a new VO. A live VO with the same name is an error rather
// than the silent overwrite UpsertVO (the importer's path) would do.
func (q *Queries) InsertVO(ctx context.Context, name string, voID int64, disable bool, raw []byte) error {
	var live bool
	if err := q.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM vos WHERE name=$1 AND deleted_at IS NULL)`, name).Scan(&live); err != nil {
		return err
	}
	if live {
		return ErrVOExists
	}
	_, err := q.pool.Exec(ctx,
		`INSERT INTO vos (name, vo_id, disable, raw_yaml) VALUES ($1,$2,$3,$4)`, name, voID, disable, string(raw))
	return err
}

// UpdateVOFields rewrites a live VO's document in place, keyed by name.
func (q *Queries) UpdateVOFields(ctx context.Context, name string, voID int64, disable bool, raw []byte) error {
	tag, err := q.pool.Exec(ctx,
		`UPDATE vos SET vo_id=$2, disable=$3, raw_yaml=$4, updated_at=NOW() WHERE name=$1 AND deleted_at IS NULL`,
		name, voID, disable, string(raw))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("VO %q not found", name)
	}
	return nil
}

// SoftDeleteVOByName soft-deletes a VO.
func (q *Queries) SoftDeleteVOByName(ctx context.Context, name, byUser string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE vos SET deleted_at=NOW(), deleted_by=$2 WHERE name=$1 AND deleted_at IS NULL`,
		name, nullString(byUser))
	return err
}

// ResourceNamesAllowingVO lists live resources whose AllowedVOs include the VO.
func (q *Queries) ResourceNamesAllowingVO(ctx context.Context, vo string) ([]string, error) {
	return q.childNames(ctx,
		`SELECT name FROM resources WHERE $1 = ANY(allowed_vos) AND deleted_at IS NULL ORDER BY name`, vo)
}

// ProjectNamesSponsoredByVO lists live projects whose sponsor is the VO.
func (q *Queries) ProjectNamesSponsoredByVO(ctx context.Context, vo string) ([]string, error) {
	return q.childNames(ctx,
		`SELECT name FROM projects WHERE sponsor_type='VirtualOrganization' AND sponsor_name=$1
		 AND deleted_at IS NULL ORDER BY name`, vo)
}

// ListReportingGroupNames returns the shared reporting-group registry's names.
func (q *Queries) ListReportingGroupNames(ctx context.Context) ([]string, error) {
	rows, err := q.ListReportingGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out, nil
}
