package db

import (
	"context"
)

// This file holds the persistence layer for the topology domain used by the
// importer/exporter (backup/restore round-trip). Rows use plain param/return
// structs; JSONB fields are passed as pre-marshaled []byte.

// ---- insert params ----

type FacilityRow struct {
	ID            string
	TopologyID    int64
	Name          string
	InstitutionID string
	Extra         []byte
	IDExplicit    bool
}

type SiteRow struct {
	ID           string
	TopologyID   int64
	FacilityID   string
	FacilityName string // populated on export
	Name         string
	LongName     string
	Description  string
	AddressLine1 string
	AddressLine2 string
	City         string
	State        string
	Country      string
	Zipcode      string
	Latitude     *float64
	Longitude    *float64
	Extra        []byte
	IDExplicit   bool
}

type ResourceGroupRow struct {
	ID               string
	GroupID          int64
	SiteID           string
	SiteName         string // populated on export
	Name             string
	Production       *bool
	SupportCenter    string
	GroupDescription string
	Extra            []byte
	IDExplicit       bool
}

type ResourceRow struct {
	ID              string
	TopologyID      int64
	ResourceGroupID string
	RGName          string // populated on export
	Name            string
	Active          *bool
	Description     string
	FQDN            string
	DN              string
	FQDNAliases     []string
	Tags            []string
	AllowedVOs      []string
	VOOwnership     []byte
	WLCGInformation []byte
	Extra           []byte
	IDExplicit      bool
}

type ResourceServiceRow struct {
	ResourceID  string
	ServiceName string
	Description string
	Details     []byte
	Ordinal     int
}

type ResourceContactRow struct {
	ResourceID  string
	ContactType string
	Rank        string
	ContactName string
	ContactID   string
}

type DowntimeRow struct {
	DtID         int64
	RGID         string
	RGName       string // populated on export
	ResourceName string
	Class        string
	Severity     string
	Description  string
	StartTime    string
	EndTime      string
	CreatedTime  string
	Services     []string
	Ordinal      int
}

// ---- services & support centers ----

// UpsertService inserts or updates a service id by name.
func (q *Queries) UpsertService(ctx context.Context, id int64, name string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO services (id, name) VALUES ($1,$2)
		 ON CONFLICT (name) DO UPDATE SET id = $1`, id, name)
	return err
}

// UpsertSupportCenter inserts or updates a support center by name.
func (q *Queries) UpsertSupportCenter(ctx context.Context, id int64, name, longName, community, description string) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO support_centers (id, name, long_name, community, description)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (name) DO UPDATE SET id=$1, long_name=$3, community=$4, description=$5`,
		id, name, nullString(longName), nullString(community), nullString(description))
	return err
}

// ServiceIDByName returns the id for a service name, or (0,false) if unknown.
func (q *Queries) ServiceIDByName(ctx context.Context, name string) (int64, bool) {
	var id int64
	if err := q.pool.QueryRow(ctx, `SELECT id FROM services WHERE name = $1`, name).Scan(&id); err != nil {
		return 0, false
	}
	return id, true
}

// SupportCenterIDByName returns the id for a support center name.
func (q *Queries) SupportCenterIDByName(ctx context.Context, name string) (int64, bool) {
	var id int64
	if err := q.pool.QueryRow(ctx, `SELECT id FROM support_centers WHERE name = $1`, name).Scan(&id); err != nil {
		return 0, false
	}
	return id, true
}

// ListAllServices returns the full services name->id map.
func (q *Queries) ListAllServices(ctx context.Context) (map[string]int64, error) {
	rows, err := q.pool.Query(ctx, `SELECT name, id FROM services ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var name string
		var id int64
		if err := rows.Scan(&name, &id); err != nil {
			return nil, err
		}
		out[name] = id
	}
	return out, rows.Err()
}

// SupportCenterFull is a full support-center row (for export).
type SupportCenterFull struct {
	ID          int64
	Name        string
	LongName    string
	Community   string
	Description string
}

// ListAllSupportCenters returns all support centers.
func (q *Queries) ListAllSupportCenters(ctx context.Context) ([]SupportCenterFull, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, name, COALESCE(long_name,''), COALESCE(community,''), COALESCE(description,'')
		 FROM support_centers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SupportCenterFull
	for rows.Next() {
		var s SupportCenterFull
		if err := rows.Scan(&s.ID, &s.Name, &s.LongName, &s.Community, &s.Description); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// TruncateTopology clears all topology-domain tables (used before a restore).
func (q *Queries) TruncateTopology(ctx context.Context) error {
	_, err := q.pool.Exec(ctx, `TRUNCATE downtimes, resource_contacts, entity_contacts,
		contact_replacements, resource_services, resources, resource_groups, sites, facilities,
		services, support_centers, vos, projects CASCADE`)
	return err
}

// ---- inserts ----

func (q *Queries) InsertFacility(ctx context.Context, r FacilityRow) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO facilities (topology_id, name, institution_id, extra, id_explicit)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		r.TopologyID, r.Name, nullString(r.InstitutionID), nullBytes(r.Extra), r.IDExplicit).Scan(&id)
	return id, err
}

func (q *Queries) InsertSite(ctx context.Context, r SiteRow) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO sites (topology_id, facility_id, name, long_name, description,
		    address_line1, address_line2, city, state, country, zipcode, latitude,
		    longitude, extra, id_explicit)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id`,
		r.TopologyID, r.FacilityID, r.Name, nullString(r.LongName), nullString(r.Description),
		nullString(r.AddressLine1), nullString(r.AddressLine2), nullString(r.City),
		nullString(r.State), nullString(r.Country), nullString(r.Zipcode),
		r.Latitude, r.Longitude, nullBytes(r.Extra), r.IDExplicit).Scan(&id)
	return id, err
}

func (q *Queries) InsertResourceGroup(ctx context.Context, r ResourceGroupRow) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO resource_groups (group_id, site_id, name, production, support_center,
		    group_description, extra, id_explicit)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		r.GroupID, r.SiteID, r.Name, r.Production, nullString(r.SupportCenter),
		nullString(r.GroupDescription), nullBytes(r.Extra), r.IDExplicit).Scan(&id)
	return id, err
}

func (q *Queries) InsertResource(ctx context.Context, r ResourceRow) (string, error) {
	var id string
	err := q.pool.QueryRow(ctx,
		`INSERT INTO resources (topology_id, resource_group_id, name, active, description,
		    fqdn, dn, fqdn_aliases, tags, allowed_vos, vo_ownership, wlcg_information,
		    extra, id_explicit)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14) RETURNING id`,
		r.TopologyID, r.ResourceGroupID, r.Name, r.Active, nullString(r.Description),
		r.FQDN, nullString(r.DN), r.FQDNAliases, r.Tags, r.AllowedVOs,
		nullBytes(r.VOOwnership), nullBytes(r.WLCGInformation), nullBytes(r.Extra),
		r.IDExplicit).Scan(&id)
	return id, err
}

func (q *Queries) InsertResourceService(ctx context.Context, r ResourceServiceRow) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO resource_services (resource_id, service_name, description, details, ordinal)
		 VALUES ($1,$2,$3,$4,$5)`,
		r.ResourceID, r.ServiceName, nullString(r.Description), nullBytes(r.Details), r.Ordinal)
	return err
}

func (q *Queries) InsertResourceContact(ctx context.Context, r ResourceContactRow) error {
	// Contacts must be users: bootstrap a provisioned (identity-less) user for
	// this contact and link it.
	userID, _ := q.UpsertProvisionedContactUser(ctx, r.ContactName, r.ContactID)
	_, err := q.pool.Exec(ctx,
		`INSERT INTO resource_contacts (resource_id, contact_type, rank, contact_name, contact_id, user_id)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		r.ResourceID, r.ContactType, r.Rank, nullString(r.ContactName), nullString(r.ContactID),
		nullString(userID))
	return err
}

func (q *Queries) InsertDowntime(ctx context.Context, r DowntimeRow) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO downtimes (dt_id, resource_group_id, resource_name, class, severity,
		    description, start_time, end_time, created_time, services, ordinal)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		r.DtID, r.RGID, r.ResourceName, r.Class, nullString(r.Severity),
		nullString(r.Description), r.StartTime, r.EndTime, nullString(r.CreatedTime),
		r.Services, r.Ordinal)
	return err
}

// ---- exports (active rows only) ----

func (q *Queries) ListFacilities(ctx context.Context) ([]FacilityRow, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, topology_id, name, COALESCE(institution_id,''), extra, id_explicit
		 FROM facilities WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FacilityRow
	for rows.Next() {
		var r FacilityRow
		if err := rows.Scan(&r.ID, &r.TopologyID, &r.Name, &r.InstitutionID, &r.Extra, &r.IDExplicit); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) ListSites(ctx context.Context) ([]SiteRow, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT s.id, s.topology_id, f.name, s.name, COALESCE(s.long_name,''),
		        COALESCE(s.description,''), COALESCE(s.address_line1,''),
		        COALESCE(s.address_line2,''), COALESCE(s.city,''), COALESCE(s.state,''),
		        COALESCE(s.country,''), COALESCE(s.zipcode,''), s.latitude, s.longitude,
		        s.extra, s.id_explicit
		 FROM sites s JOIN facilities f ON f.id = s.facility_id
		 WHERE s.deleted_at IS NULL ORDER BY s.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SiteRow
	for rows.Next() {
		var r SiteRow
		if err := rows.Scan(&r.ID, &r.TopologyID, &r.FacilityName, &r.Name, &r.LongName,
			&r.Description, &r.AddressLine1, &r.AddressLine2, &r.City, &r.State,
			&r.Country, &r.Zipcode, &r.Latitude, &r.Longitude, &r.Extra, &r.IDExplicit); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) ListResourceGroups(ctx context.Context) ([]ResourceGroupRow, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT rg.id, rg.group_id, s.name, rg.name, rg.production,
		        COALESCE(rg.support_center,''), COALESCE(rg.group_description,''),
		        rg.extra, rg.id_explicit
		 FROM resource_groups rg JOIN sites s ON s.id = rg.site_id
		 WHERE rg.deleted_at IS NULL ORDER BY rg.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResourceGroupRow
	for rows.Next() {
		var r ResourceGroupRow
		if err := rows.Scan(&r.ID, &r.GroupID, &r.SiteName, &r.Name, &r.Production,
			&r.SupportCenter, &r.GroupDescription, &r.Extra, &r.IDExplicit); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) ListResources(ctx context.Context) ([]ResourceRow, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT r.id, r.topology_id, rg.name, r.name, r.active, COALESCE(r.description,''),
		        r.fqdn, COALESCE(r.dn,''), r.fqdn_aliases, r.tags, r.allowed_vos,
		        r.vo_ownership, r.wlcg_information, r.extra, r.id_explicit
		 FROM resources r JOIN resource_groups rg ON rg.id = r.resource_group_id
		 WHERE r.deleted_at IS NULL ORDER BY r.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResourceRow
	for rows.Next() {
		var r ResourceRow
		if err := rows.Scan(&r.ID, &r.TopologyID, &r.RGName, &r.Name, &r.Active, &r.Description,
			&r.FQDN, &r.DN, &r.FQDNAliases, &r.Tags, &r.AllowedVOs,
			&r.VOOwnership, &r.WLCGInformation, &r.Extra, &r.IDExplicit); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) ListResourceServices(ctx context.Context, resourceID string) ([]ResourceServiceRow, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT service_name, COALESCE(description,''), details
		 FROM resource_services WHERE resource_id = $1 ORDER BY ordinal, service_name`, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResourceServiceRow
	for rows.Next() {
		r := ResourceServiceRow{ResourceID: resourceID}
		if err := rows.Scan(&r.ServiceName, &r.Description, &r.Details); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) ListResourceContacts(ctx context.Context, resourceID string) ([]ResourceContactRow, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT contact_type, rank, COALESCE(contact_name,''), COALESCE(contact_id,'')
		 FROM resource_contacts WHERE resource_id = $1 AND deleted_at IS NULL`, resourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ResourceContactRow
	for rows.Next() {
		r := ResourceContactRow{ResourceID: resourceID}
		if err := rows.Scan(&r.ContactType, &r.Rank, &r.ContactName, &r.ContactID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) ListDowntimes(ctx context.Context) ([]DowntimeRow, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT d.dt_id, rg.name, d.resource_name, d.class, COALESCE(d.severity,''),
		        COALESCE(d.description,''), d.start_time, d.end_time,
		        COALESCE(d.created_time,''), d.services
		 FROM downtimes d JOIN resource_groups rg ON rg.id = d.resource_group_id
		 WHERE d.deleted_at IS NULL ORDER BY rg.name, d.ordinal, d.dt_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DowntimeRow
	for rows.Next() {
		var r DowntimeRow
		if err := rows.Scan(&r.DtID, &r.RGName, &r.ResourceName, &r.Class, &r.Severity,
			&r.Description, &r.StartTime, &r.EndTime, &r.CreatedTime, &r.Services); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
