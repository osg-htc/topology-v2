package db

import "context"

// Summary holds active-entity counts for the dashboard.
type Summary struct {
	Resources      int `json:"resources"`
	ResourceGroups int `json:"resource_groups"`
	Sites          int `json:"sites"`
	Facilities     int `json:"facilities"`
	Institutions   int `json:"institutions"`
	VOs            int `json:"vos"`
	Projects       int `json:"projects"`
}

// CountsSummary returns counts of active (non-soft-deleted) entities.
func (q *Queries) CountsSummary(ctx context.Context) (Summary, error) {
	var s Summary
	err := q.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM resources WHERE deleted_at IS NULL),
		(SELECT count(*) FROM resource_groups WHERE deleted_at IS NULL),
		(SELECT count(*) FROM sites WHERE deleted_at IS NULL),
		(SELECT count(*) FROM facilities WHERE deleted_at IS NULL),
		(SELECT count(*) FROM institutions),
		(SELECT count(*) FROM vos WHERE deleted_at IS NULL),
		(SELECT count(*) FROM projects WHERE deleted_at IS NULL)`).
		Scan(&s.Resources, &s.ResourceGroups, &s.Sites, &s.Facilities,
			&s.Institutions, &s.VOs, &s.Projects)
	return s, err
}

// BrowseRG is a resource-group list row for the management UI.
type BrowseRG struct {
	Name             string `json:"name"`
	GroupID          int64  `json:"group_id"`
	Site             string `json:"site"`
	Facility         string `json:"facility"`
	Production       *bool  `json:"production"`
	SupportCenter    string `json:"support_center"`
	GroupDescription string `json:"group_description"`
	ResourceCount    int    `json:"resource_count"`
	Deleted          bool   `json:"deleted"`
}

// ListBrowseRGs lists resource groups (active only unless includeDeleted).
func (q *Queries) ListBrowseRGs(ctx context.Context, includeDeleted bool) ([]BrowseRG, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT rg.name, rg.group_id, s.name, f.name, rg.production,
		        COALESCE(rg.support_center,''), COALESCE(rg.group_description,''),
		        (SELECT count(*) FROM resources r WHERE r.resource_group_id = rg.id AND r.deleted_at IS NULL),
		        rg.deleted_at IS NOT NULL
		 FROM resource_groups rg
		 JOIN sites s ON s.id = rg.site_id
		 JOIN facilities f ON f.id = s.facility_id
		 WHERE ($1 OR rg.deleted_at IS NULL)
		 ORDER BY rg.name`, includeDeleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BrowseRG, 0)
	for rows.Next() {
		var r BrowseRG
		if err := rows.Scan(&r.Name, &r.GroupID, &r.Site, &r.Facility, &r.Production,
			&r.SupportCenter, &r.GroupDescription, &r.ResourceCount, &r.Deleted); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BrowseSite is a site list row.
type BrowseSite struct {
	Name        string `json:"name"`
	SiteID      int64  `json:"site_id"`
	Facility    string `json:"facility"`
	LongName    string `json:"long_name"`
	City        string `json:"city"`
	State       string `json:"state"`
	Country     string `json:"country"`
	Deleted     bool   `json:"deleted"`
}

// ListBrowseSites lists sites (active only unless includeDeleted).
func (q *Queries) ListBrowseSites(ctx context.Context, includeDeleted bool) ([]BrowseSite, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT s.name, s.topology_id, f.name, COALESCE(s.long_name,''),
		        COALESCE(s.city,''), COALESCE(s.state,''), COALESCE(s.country,''),
		        s.deleted_at IS NOT NULL
		 FROM sites s JOIN facilities f ON f.id = s.facility_id
		 WHERE ($1 OR s.deleted_at IS NULL)
		 ORDER BY s.name`, includeDeleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BrowseSite, 0)
	for rows.Next() {
		var r BrowseSite
		if err := rows.Scan(&r.Name, &r.SiteID, &r.Facility, &r.LongName,
			&r.City, &r.State, &r.Country, &r.Deleted); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BrowseFacility is a facility list row.
type BrowseFacility struct {
	Name          string `json:"name"`
	FacilityID    int64  `json:"facility_id"`
	InstitutionID string `json:"institution_id"`
	SiteCount     int    `json:"site_count"`
	Deleted       bool   `json:"deleted"`
}

// ListBrowseFacilities lists facilities (active only unless includeDeleted).
func (q *Queries) ListBrowseFacilities(ctx context.Context, includeDeleted bool) ([]BrowseFacility, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT f.name, f.topology_id, COALESCE(f.institution_id,''),
		        (SELECT count(*) FROM sites s WHERE s.facility_id = f.id AND s.deleted_at IS NULL),
		        f.deleted_at IS NOT NULL
		 FROM facilities f
		 WHERE ($1 OR f.deleted_at IS NULL)
		 ORDER BY f.name`, includeDeleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BrowseFacility, 0)
	for rows.Next() {
		var r BrowseFacility
		if err := rows.Scan(&r.Name, &r.FacilityID, &r.InstitutionID, &r.SiteCount, &r.Deleted); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BrowseProject is a project list row.
type BrowseProject struct {
	Name           string `json:"name"`
	ProjectID      string `json:"project_id"`
	PIName         string `json:"pi_name"`
	Organization   string `json:"organization"`
	FieldOfScience string `json:"field_of_science"`
	SponsorType    string `json:"sponsor_type"`
	SponsorName    string `json:"sponsor_name"`
	Deleted        bool   `json:"deleted"`
}

// ListBrowseProjects lists projects (active only unless includeDeleted).
func (q *Queries) ListBrowseProjects(ctx context.Context, includeDeleted bool) ([]BrowseProject, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT name, COALESCE(project_id,''), COALESCE(pi_name,''), COALESCE(organization,''),
		        COALESCE(field_of_science,''), COALESCE(sponsor_type,''), COALESCE(sponsor_name,''),
		        deleted_at IS NOT NULL
		 FROM projects WHERE ($1 OR deleted_at IS NULL) ORDER BY name`, includeDeleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BrowseProject, 0)
	for rows.Next() {
		var r BrowseProject
		if err := rows.Scan(&r.Name, &r.ProjectID, &r.PIName, &r.Organization,
			&r.FieldOfScience, &r.SponsorType, &r.SponsorName, &r.Deleted); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetProjectByName returns a single project's full row (for detail/edit).
func (q *Queries) GetProjectByName(ctx context.Context, name string) (*ProjectRow, error) {
	r := &ProjectRow{}
	err := q.pool.QueryRow(ctx,
		`SELECT name, COALESCE(project_id,''), COALESCE(description,''), COALESCE(department,''),
		        COALESCE(field_of_science,''), COALESCE(field_of_science_id,''),
		        COALESCE(organization,''), COALESCE(pi_name,''), COALESCE(institution_id,''),
		        sponsor, COALESCE(sponsor_type,''), COALESCE(sponsor_name,''), extra
		 FROM projects WHERE name=$1 AND deleted_at IS NULL`, name).
		Scan(&r.Name, &r.ProjectID, &r.Description, &r.Department, &r.FieldOfScience,
			&r.FieldOfScienceID, &r.Organization, &r.PIName, &r.InstitutionID,
			&r.Sponsor, &r.SponsorType, &r.SponsorName, &r.Extra)
	if err != nil {
		return nil, ErrNotFound
	}
	return r, nil
}

// SoftDeleteProjectByName soft-deletes a project.
func (q *Queries) SoftDeleteProjectByName(ctx context.Context, name, byUser string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE projects SET deleted_at=NOW(), deleted_by=$2 WHERE name=$1 AND deleted_at IS NULL`,
		name, nullString(byUser))
	return err
}

// Institution is a cached institution registry row.
type Institution struct {
	IIDURI string `json:"id"`
	Name   string `json:"name"`
	RORID  string `json:"ror_id"`
}

// ListInstitutions returns cached institutions ordered by name.
func (q *Queries) ListInstitutions(ctx context.Context) ([]Institution, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT iid_uri, name, COALESCE(ror_id,'') FROM institutions ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Institution, 0)
	for rows.Next() {
		var i Institution
		if err := rows.Scan(&i.IIDURI, &i.Name, &i.RORID); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// UpsertInstitution caches an institution (aggressive cache: ids are immutable).
func (q *Queries) UpsertInstitution(ctx context.Context, iidURI, name, rorID string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO institutions (iid_uri, name, ror_id, cached_at) VALUES ($1,$2,$3,NOW())
		 ON CONFLICT (iid_uri) DO UPDATE SET name=$2, ror_id=$3, cached_at=NOW()`,
		iidURI, name, nullString(rorID))
	return err
}

// ---- name lookups + soft deletes for proposal apply ----

// FacilityIDByName returns the active facility id for a name.
func (q *Queries) FacilityIDByName(ctx context.Context, name string) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`SELECT id FROM facilities WHERE name=$1 AND deleted_at IS NULL`, name).Scan(&id)
	if err != nil {
		return "", ErrNotFound
	}
	return id, nil
}

// SiteIDByName returns the active site id for a name.
func (q *Queries) SiteIDByName(ctx context.Context, name string) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`SELECT id FROM sites WHERE name=$1 AND deleted_at IS NULL`, name).Scan(&id)
	if err != nil {
		return "", ErrNotFound
	}
	return id, nil
}

// SoftDeleteResourceGroupByName soft-deletes a resource group.
func (q *Queries) SoftDeleteResourceGroupByName(ctx context.Context, name, byUser string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE resource_groups SET deleted_at=NOW(), deleted_by=$2 WHERE name=$1 AND deleted_at IS NULL`,
		name, nullString(byUser))
	return err
}

// SoftDeleteSiteByName soft-deletes a site.
func (q *Queries) SoftDeleteSiteByName(ctx context.Context, name, byUser string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE sites SET deleted_at=NOW(), deleted_by=$2 WHERE name=$1 AND deleted_at IS NULL`,
		name, nullString(byUser))
	return err
}

// SoftDeleteFacilityByName soft-deletes a facility.
func (q *Queries) SoftDeleteFacilityByName(ctx context.Context, name, byUser string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE facilities SET deleted_at=NOW(), deleted_by=$2 WHERE name=$1 AND deleted_at IS NULL`,
		name, nullString(byUser))
	return err
}

// UpdateResourceGroupFields updates a resource group in place (preserving its id
// and child resources). Moving to a different site is supported via siteID.
func (q *Queries) UpdateResourceGroupFields(ctx context.Context, name, siteID string, production *bool, supportCenter, groupDescription string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE resource_groups
		 SET site_id=$2, production=$3, support_center=$4, group_description=$5, updated_at=NOW()
		 WHERE name=$1 AND deleted_at IS NULL`,
		name, siteID, production, nullString(supportCenter), nullString(groupDescription))
	return err
}

// UpdateSiteFields updates a site in place.
func (q *Queries) UpdateSiteFields(ctx context.Context, r SiteRow) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE sites SET facility_id=$2, long_name=$3, description=$4, address_line1=$5,
		    address_line2=$6, city=$7, state=$8, country=$9, zipcode=$10, latitude=$11,
		    longitude=$12, updated_at=NOW()
		 WHERE name=$1 AND deleted_at IS NULL`,
		r.Name, r.FacilityID, nullString(r.LongName), nullString(r.Description),
		nullString(r.AddressLine1), nullString(r.AddressLine2), nullString(r.City),
		nullString(r.State), nullString(r.Country), nullString(r.Zipcode), r.Latitude, r.Longitude)
	return err
}

// UpdateFacilityFields updates a facility in place.
func (q *Queries) UpdateFacilityFields(ctx context.Context, name, institutionID string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE facilities SET institution_id=$2, updated_at=NOW()
		 WHERE name=$1 AND deleted_at IS NULL`,
		name, nullString(institutionID))
	return err
}
