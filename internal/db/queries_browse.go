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
	Name     string `json:"name"`
	SiteID   int64  `json:"site_id"`
	Facility string `json:"facility"`
	LongName string `json:"long_name"`
	City     string `json:"city"`
	State    string `json:"state"`
	Country  string `json:"country"`
	Deleted  bool   `json:"deleted"`
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

// ResourceDetail is the full detail of a single resource (for the detail page).
type ResourceDetail struct {
	Name            string   `json:"name"`
	TopologyID      int64    `json:"id"`
	ResourceGroup   string   `json:"resource_group"`
	Site            string   `json:"site"`
	Facility        string   `json:"facility"`
	Active          *bool    `json:"active"`
	Description     string   `json:"description"`
	FQDN            string   `json:"fqdn"`
	DN              string   `json:"dn"`
	FQDNAliases     []string `json:"fqdn_aliases"`
	Tags            []string `json:"tags"`
	AllowedVOs      []string `json:"allowed_vos"`
	VOOwnership     []byte   `json:"-"`
	WLCGInformation []byte   `json:"-"`
	Deleted         bool     `json:"deleted"`
}

// GetResourceDetail returns a single resource (active), looked up by its
// immutable topology_id rather than name -- a rename must never change what
// URL/link reaches a resource.
func (q *Queries) GetResourceDetail(ctx context.Context, topologyID int64) (*ResourceDetail, error) {
	d := &ResourceDetail{}
	err := q.pool.QueryRow(ctx,
		`SELECT r.name, r.topology_id, rg.name, s.name, f.name, r.active,
		        COALESCE(r.description,''), r.fqdn, COALESCE(r.dn,''),
		        r.fqdn_aliases, r.tags, r.allowed_vos, r.vo_ownership, r.wlcg_information
		 FROM resources r
		 JOIN resource_groups rg ON rg.id = r.resource_group_id
		 JOIN sites s ON s.id = rg.site_id
		 JOIN facilities f ON f.id = s.facility_id
		 WHERE r.topology_id = $1 AND r.deleted_at IS NULL`, topologyID).
		Scan(&d.Name, &d.TopologyID, &d.ResourceGroup, &d.Site, &d.Facility, &d.Active,
			&d.Description, &d.FQDN, &d.DN, &d.FQDNAliases, &d.Tags, &d.AllowedVOs,
			&d.VOOwnership, &d.WLCGInformation)
	if err != nil {
		return nil, ErrNotFound
	}
	return d, nil
}

// ResourceIDForName returns the resource's topology_id (active) for contact/
// service lookups.
func (q *Queries) ResourceIDForName(ctx context.Context, name string) (int64, error) {
	return q.ResourceIDByName(ctx, name)
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
		        sponsor, COALESCE(sponsor_type,''), COALESCE(sponsor_name,''), extra, updated_at
		 FROM projects WHERE name=$1 AND deleted_at IS NULL`, name).
		Scan(&r.Name, &r.ProjectID, &r.Description, &r.Department, &r.FieldOfScience,
			&r.FieldOfScienceID, &r.Organization, &r.PIName, &r.InstitutionID,
			&r.Sponsor, &r.SponsorType, &r.SponsorName, &r.Extra, &r.UpdatedAt)
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

// childNames runs a name-listing query and collects the results.
func (q *Queries) childNames(ctx context.Context, sql string, arg string) ([]string, error) {
	rows, err := q.pool.Query(ctx, sql, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ResourceNamesInRG lists active resource names in a resource group.
func (q *Queries) ResourceNamesInRG(ctx context.Context, rgName string) ([]string, error) {
	return q.childNames(ctx,
		`SELECT r.name FROM resources r JOIN resource_groups rg ON rg.id = r.resource_group_id
		 WHERE rg.name=$1 AND r.deleted_at IS NULL ORDER BY r.name`, rgName)
}

// RGNamesInSite lists active resource-group names at a site.
func (q *Queries) RGNamesInSite(ctx context.Context, siteName string) ([]string, error) {
	return q.childNames(ctx,
		`SELECT rg.name FROM resource_groups rg JOIN sites s ON s.id = rg.site_id
		 WHERE s.name=$1 AND rg.deleted_at IS NULL ORDER BY rg.name`, siteName)
}

// SiteNamesInFacility lists active site names in a facility.
func (q *Queries) SiteNamesInFacility(ctx context.Context, facName string) ([]string, error) {
	return q.childNames(ctx,
		`SELECT s.name FROM sites s JOIN facilities f ON f.id = s.facility_id
		 WHERE f.name=$1 AND s.deleted_at IS NULL ORDER BY s.name`, facName)
}

// GetSiteDetail returns a single site's full fields with its facility name.
func (q *Queries) GetSiteDetail(ctx context.Context, name string) (*SiteRow, string, error) {
	r := &SiteRow{}
	var facName string
	err := q.pool.QueryRow(ctx,
		`SELECT s.name, f.name, COALESCE(s.long_name,''), COALESCE(s.description,''),
		        COALESCE(s.address_line1,''), COALESCE(s.address_line2,''), COALESCE(s.city,''),
		        COALESCE(s.state,''), COALESCE(s.country,''), COALESCE(s.zipcode,''),
		        s.latitude, s.longitude
		 FROM sites s JOIN facilities f ON f.id = s.facility_id
		 WHERE s.name=$1 AND s.deleted_at IS NULL`, name).
		Scan(&r.Name, &facName, &r.LongName, &r.Description, &r.AddressLine1, &r.AddressLine2,
			&r.City, &r.State, &r.Country, &r.Zipcode, &r.Latitude, &r.Longitude)
	if err != nil {
		return nil, "", ErrNotFound
	}
	return r, facName, nil
}

// nameList runs a no-argument name query and collects the results.
func (q *Queries) nameList(ctx context.Context, sql string) ([]string, error) {
	rows, err := q.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ListServiceNames returns known service names (services.yaml), sorted.
func (q *Queries) ListServiceNames(ctx context.Context) ([]string, error) {
	return q.nameList(ctx, `SELECT name FROM services ORDER BY name`)
}

// ListVONames returns active VO names, sorted.
func (q *Queries) ListVONames(ctx context.Context) ([]string, error) {
	return q.nameList(ctx, `SELECT name FROM vos WHERE deleted_at IS NULL ORDER BY name`)
}

// ListDistinctTags returns the distinct resource tags currently in use.
func (q *Queries) ListDistinctTags(ctx context.Context) ([]string, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT DISTINCT unnest(tags) AS t FROM resources WHERE deleted_at IS NULL ORDER BY t`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ResourceRGID returns the active resource group id for a resource name.
func (q *Queries) ResourceRGID(ctx context.Context, resourceName string) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`SELECT rg.id FROM resources r JOIN resource_groups rg ON rg.id = r.resource_group_id
		 WHERE r.name = $1 AND r.deleted_at IS NULL`, resourceName).Scan(&id)
	if err != nil {
		return "", ErrNotFound
	}
	return id, nil
}

// GetDowntimeResourceName returns the resource a downtime targets, by its
// dt_id -- used to resolve who may decide an update/delete proposal for an
// existing downtime (see IsResourceContact in queries_contacts.go).
func (q *Queries) GetDowntimeResourceName(ctx context.Context, dtID int64) (string, error) {
	var name string
	err := q.pool.QueryRow(ctx,
		`SELECT resource_name FROM downtimes WHERE dt_id=$1 AND deleted_at IS NULL`, dtID).Scan(&name)
	if err != nil {
		return "", ErrNotFound
	}
	return name, nil
}

// UpdateDowntimeByID updates a downtime in place (identified by its dt_id).
func (q *Queries) UpdateDowntimeByID(ctx context.Context, dtID int64, class, severity, description, start, end string, services []string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE downtimes SET class=$2, severity=$3, description=$4, start_time=$5,
		    end_time=$6, services=$7
		 WHERE dt_id=$1 AND deleted_at IS NULL`,
		dtID, class, nullString(severity), nullString(description), start, end, services)
	return err
}

// SoftDeleteDowntimeByID soft-deletes a downtime.
func (q *Queries) SoftDeleteDowntimeByID(ctx context.Context, dtID int64, byUser string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE downtimes SET deleted_at=NOW(), deleted_by=$2 WHERE dt_id=$1 AND deleted_at IS NULL`,
		dtID, nullString(byUser))
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

// SearchInstitutions returns cached institutions whose name matches q (all when
// q is empty), capped at limit. Backs the facility institution picker.
func (q *Queries) SearchInstitutions(ctx context.Context, query string, limit int) ([]Institution, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := q.pool.Query(ctx,
		`SELECT iid_uri, name, COALESCE(ror_id,'') FROM institutions
		 WHERE $1 = '' OR name ILIKE '%' || $1 || '%'
		 ORDER BY name LIMIT $2`, query, limit)
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

// InstitutionExists reports whether an institution id is in the cache.
func (q *Queries) InstitutionExists(ctx context.Context, iid string) (bool, error) {
	var ok bool
	err := q.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM institutions WHERE iid_uri = $1)`, iid).Scan(&ok)
	return ok, err
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

// UpdateResourceFields updates a resource in place, keyed by its immutable
// topology_id rather than name -- a rename must never change identity or
// orphan resource_services/resource_contacts (both FK on resource_id =
// resources.topology_id). Does not touch services/contacts; callers replace
// those separately (see ReplaceResourceServices/ReplaceResourceContacts).
func (q *Queries) UpdateResourceFields(ctx context.Context, r ResourceRow) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE resources SET resource_group_id=$2, name=$3, active=$4, description=$5,
		    fqdn=$6, dn=$7, fqdn_aliases=$8, tags=$9, allowed_vos=$10, vo_ownership=$11,
		    wlcg_information=$12, extra=$13, updated_at=NOW()
		 WHERE topology_id=$1 AND deleted_at IS NULL`,
		r.TopologyID, r.ResourceGroupID, r.Name, r.Active, nullString(r.Description),
		r.FQDN, nullString(r.DN), r.FQDNAliases, r.Tags, r.AllowedVOs,
		nullBytes(r.VOOwnership), nullBytes(r.WLCGInformation), nullBytes(r.Extra))
	return err
}

// ReplaceResourceServices sets the full Services set for a resource: hard-
// deletes the current rows (resource_services has no deleted_at -- it is pure
// child aggregate data, fully recomputed from each edit's payload) and lets
// the caller re-insert via InsertResourceService.
func (q *Queries) ReplaceResourceServices(ctx context.Context, resourceID int64) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM resource_services WHERE resource_id=$1`, resourceID)
	return err
}

// ReplaceResourceContactsBegin soft-deletes a resource's current contact set,
// mirroring ReplaceEntityContacts' pattern for resource_group/site/facility.
// The caller re-inserts via InsertResourceContact.
func (q *Queries) ReplaceResourceContactsBegin(ctx context.Context, resourceID int64, byUser string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE resource_contacts SET deleted_at = NOW(), deleted_by = $2
		 WHERE resource_id = $1 AND deleted_at IS NULL`,
		resourceID, nullString(byUser))
	return err
}

// UpdateResourceGroupFields updates a resource group in place (preserving its id
// and child resources). Moving to a different site is supported via siteID.
// UpdateResourceGroupFields updates a resource group in place, keyed by
// targetName (its name before this edit) rather than the new name -- a
// rename must locate the existing row by what it WAS called, not by what
// it's being renamed TO (which, on a rename, doesn't exist yet and would
// silently match zero rows).
func (q *Queries) UpdateResourceGroupFields(ctx context.Context, targetName, newName, siteID string, production *bool, supportCenter, groupDescription string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE resource_groups
		 SET name=$2, site_id=$3, production=$4, support_center=$5, group_description=$6, updated_at=NOW()
		 WHERE name=$1 AND deleted_at IS NULL`,
		targetName, newName, siteID, production, nullString(supportCenter), nullString(groupDescription))
	return err
}

// UpdateSiteFields updates a site in place.
// UpdateSiteFields updates a site in place, keyed by targetName (its name
// before this edit) -- r.Name is the new name, written into SET, not used for
// the lookup. See UpdateResourceGroupFields for why.
func (q *Queries) UpdateSiteFields(ctx context.Context, targetName string, r SiteRow) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE sites SET name=$2, facility_id=$3, long_name=$4, description=$5, address_line1=$6,
		    address_line2=$7, city=$8, state=$9, country=$10, zipcode=$11, latitude=$12,
		    longitude=$13, updated_at=NOW()
		 WHERE name=$1 AND deleted_at IS NULL`,
		targetName, r.Name, r.FacilityID, nullString(r.LongName), nullString(r.Description),
		nullString(r.AddressLine1), nullString(r.AddressLine2), nullString(r.City),
		nullString(r.State), nullString(r.Country), nullString(r.Zipcode), r.Latitude, r.Longitude)
	return err
}

// UpdateFacilityFields updates a facility in place.
// UpdateFacilityFields updates a facility in place, keyed by targetName (its
// name before this edit) -- newName is written into SET, not used for the
// lookup. See UpdateResourceGroupFields for why.
func (q *Queries) UpdateFacilityFields(ctx context.Context, targetName, newName, institutionID string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE facilities SET name=$2, institution_id=$3, updated_at=NOW()
		 WHERE name=$1 AND deleted_at IS NULL`,
		targetName, newName, nullString(institutionID))
	return err
}
