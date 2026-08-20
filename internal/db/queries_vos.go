package db

import "context"

// VORow is a virtual organization document row.
type VORow struct {
	Name    string
	VOID    int64
	Disable bool
	Raw     []byte
}

// ProjectRow is a relational project row.
type ProjectRow struct {
	Name             string
	ProjectID        string
	Description      string
	Department       string
	FieldOfScience   string
	FieldOfScienceID string
	Organization     string
	PIName           string
	InstitutionID    string
	Sponsor          []byte // JSON of the Sponsor block
	SponsorType      string
	SponsorName      string
	Extra            []byte
}

// UpsertVO inserts or updates an active VO by name.
func (q *Queries) UpsertVO(ctx context.Context, name string, voID int64, disable bool, raw []byte) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO vos (name, vo_id, disable, raw_yaml) VALUES ($1,$2,$3,$4)
		 ON CONFLICT (name) WHERE deleted_at IS NULL
		 DO UPDATE SET vo_id=$2, disable=$3, raw_yaml=$4, updated_at=NOW()`,
		name, voID, disable, string(raw))
	return err
}

// ListVOs returns all active VOs, ordered by name.
func (q *Queries) ListVOs(ctx context.Context) ([]VORow, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT name, vo_id, disable, raw_yaml FROM vos WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VORow
	for rows.Next() {
		var r VORow
		var raw string
		if err := rows.Scan(&r.Name, &r.VOID, &r.Disable, &raw); err != nil {
			return nil, err
		}
		r.Raw = []byte(raw)
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpsertProject inserts or updates an active project by name.
func (q *Queries) UpsertProject(ctx context.Context, r ProjectRow) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO projects (name, project_id, description, department, field_of_science,
		    field_of_science_id, organization, pi_name, institution_id, sponsor,
		    sponsor_type, sponsor_name, extra)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 ON CONFLICT (name) WHERE deleted_at IS NULL
		 DO UPDATE SET project_id=$2, description=$3, department=$4, field_of_science=$5,
		    field_of_science_id=$6, organization=$7, pi_name=$8, institution_id=$9,
		    sponsor=$10, sponsor_type=$11, sponsor_name=$12, extra=$13, updated_at=NOW()`,
		r.Name, nullString(r.ProjectID), nullString(r.Description), nullString(r.Department),
		nullString(r.FieldOfScience), nullString(r.FieldOfScienceID), nullString(r.Organization),
		nullString(r.PIName), nullString(r.InstitutionID), nullBytes(r.Sponsor),
		nullString(r.SponsorType), nullString(r.SponsorName), nullBytes(r.Extra))
	return err
}

// UpdateProjectFields updates an existing active project in place, keyed by
// its original name (targetName), allowing the name itself to change.
func (q *Queries) UpdateProjectFields(ctx context.Context, targetName string, r ProjectRow) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE projects SET name=$2, project_id=$3, description=$4, department=$5,
		    field_of_science=$6, field_of_science_id=$7, organization=$8, pi_name=$9,
		    institution_id=$10, sponsor=$11, sponsor_type=$12, sponsor_name=$13,
		    extra=$14, updated_at=NOW()
		 WHERE name=$1 AND deleted_at IS NULL`,
		targetName, r.Name, nullString(r.ProjectID), nullString(r.Description), nullString(r.Department),
		nullString(r.FieldOfScience), nullString(r.FieldOfScienceID), nullString(r.Organization),
		nullString(r.PIName), nullString(r.InstitutionID), nullBytes(r.Sponsor),
		nullString(r.SponsorType), nullString(r.SponsorName), nullBytes(r.Extra))
	return err
}

// ListProjects returns all active projects, ordered by name.
func (q *Queries) ListProjects(ctx context.Context) ([]ProjectRow, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT name, COALESCE(project_id,''), COALESCE(description,''), COALESCE(department,''),
		        COALESCE(field_of_science,''), COALESCE(field_of_science_id,''),
		        COALESCE(organization,''), COALESCE(pi_name,''), COALESCE(institution_id,''),
		        sponsor, COALESCE(sponsor_type,''), COALESCE(sponsor_name,''), extra
		 FROM projects WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectRow
	for rows.Next() {
		var r ProjectRow
		if err := rows.Scan(&r.Name, &r.ProjectID, &r.Description, &r.Department,
			&r.FieldOfScience, &r.FieldOfScienceID, &r.Organization, &r.PIName,
			&r.InstitutionID, &r.Sponsor, &r.SponsorType, &r.SponsorName, &r.Extra); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReportingGroupRow is one entry in the shared reporting-groups registry
// (imported from virtual-organizations/REPORTING_GROUPS.yaml), cross-
// referenced by name from a VO's ReportingGroups list.
type ReportingGroupRow struct {
	Name     string
	Contacts []byte // JSON array of {ID, Name}
	FQANs    []byte // JSON array of {GroupName, Role}
}

// UpsertReportingGroup inserts or replaces one reporting group by name.
func (q *Queries) UpsertReportingGroup(ctx context.Context, name string, contacts, fqans []byte) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO reporting_groups (name, contacts, fqans)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (name) DO UPDATE SET contacts=$2, fqans=$3, updated_at=NOW()`,
		name, nullBytes(contacts), nullBytes(fqans))
	return err
}

// ListReportingGroups returns the full reporting-groups registry.
func (q *Queries) ListReportingGroups(ctx context.Context) ([]ReportingGroupRow, error) {
	rows, err := q.pool.Query(ctx, `SELECT name, contacts, fqans FROM reporting_groups ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReportingGroupRow
	for rows.Next() {
		var r ReportingGroupRow
		if err := rows.Scan(&r.Name, &r.Contacts, &r.FQANs); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CampusGridRow is one entry in the shared campus-grid sponsor registry
// (imported from projects/_CAMPUS_GRIDS.yaml), cross-referenced by name from
// a project's Sponsor.CampusGrid.
type CampusGridRow struct {
	Name         string
	CampusGridID int64
}

// UpsertCampusGrid inserts or replaces one campus grid by name.
func (q *Queries) UpsertCampusGrid(ctx context.Context, name string, id int64) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO campus_grids (name, campus_grid_id) VALUES ($1,$2)
		 ON CONFLICT (name) DO UPDATE SET campus_grid_id=$2, updated_at=NOW()`,
		name, id)
	return err
}

// ListCampusGrids returns the full campus-grid registry.
func (q *Queries) ListCampusGrids(ctx context.Context) ([]CampusGridRow, error) {
	rows, err := q.pool.Query(ctx, `SELECT name, campus_grid_id FROM campus_grids ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CampusGridRow
	for rows.Next() {
		var r CampusGridRow
		if err := rows.Scan(&r.Name, &r.CampusGridID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
