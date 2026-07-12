package topology

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/bbockelm/topology-v2/internal/db"
)

// Import writes a whole Topology into the database. It assumes an empty (or
// truncated) topology domain — it is the restore side of backup/restore.
func Import(ctx context.Context, q *db.Queries, t *Topology) error {
	facIDs := map[string]string{}
	for name, fac := range t.Facilities {
		topID, explicit := resolveID(fac.ID, name)
		id, err := q.InsertFacility(ctx, db.FacilityRow{
			TopologyID: topID, Name: name, InstitutionID: fac.InstitutionID,
			Extra: mustJSON(fac.Extra), IDExplicit: explicit,
		})
		if err != nil {
			return fmt.Errorf("insert facility %q: %w", name, err)
		}
		facIDs[name] = id
	}

	siteIDs := map[string]string{}
	for name, site := range t.Sites {
		facName := t.SiteFacility[name]
		topID, explicit := resolveID(site.ID, name)
		id, err := q.InsertSite(ctx, db.SiteRow{
			TopologyID: topID, FacilityID: facIDs[facName], Name: name,
			LongName: site.LongName, Description: site.Description,
			AddressLine1: site.AddressLine1, AddressLine2: site.AddressLine2,
			City: site.City, State: site.State, Country: site.Country,
			Zipcode: site.Zipcode, Latitude: site.Latitude, Longitude: site.Longitude,
			Extra: mustJSON(site.Extra), IDExplicit: explicit,
		})
		if err != nil {
			return fmt.Errorf("insert site %q: %w", name, err)
		}
		siteIDs[name] = id
	}

	rgIDs := map[string]string{}
	for name, rg := range t.ResourceGroups {
		siteName := t.RGSite[name]
		topID, explicit := resolveID(rg.GroupID, name)
		id, err := q.InsertResourceGroup(ctx, db.ResourceGroupRow{
			GroupID: topID, SiteID: siteIDs[siteName], Name: name,
			Production: rg.Production, SupportCenter: rg.SupportCenter,
			GroupDescription: rg.GroupDescription, Extra: mustJSON(rg.Extra),
			IDExplicit: explicit,
		})
		if err != nil {
			return fmt.Errorf("insert resource group %q: %w", name, err)
		}
		rgIDs[name] = id

		for resName, res := range rg.Resources {
			if _, err := UpsertResource(ctx, q, id, resName, res); err != nil {
				return err
			}
		}
	}

	for rgName, dts := range t.Downtimes {
		rgID := rgIDs[rgName]
		for i, d := range dts {
			if err := q.InsertDowntime(ctx, db.DowntimeRow{
				DtID: d.ID, RGID: rgID, ResourceName: d.ResourceName, Class: d.Class,
				Severity: d.Severity, Description: d.Description, StartTime: d.StartTime,
				EndTime: d.EndTime, CreatedTime: d.CreatedTime, Services: d.Services,
				Ordinal: i,
			}); err != nil {
				return fmt.Errorf("insert downtime %d: %w", d.ID, err)
			}
		}
	}
	return nil
}

// ImportServices loads the services.yaml name->id map into the services table.
func ImportServices(ctx context.Context, q *db.Queries, services map[string]int64) error {
	for name, id := range services {
		if id == 0 {
			id = GenID(name)
		}
		if err := q.UpsertService(ctx, id, name); err != nil {
			return fmt.Errorf("upsert service %q: %w", name, err)
		}
	}
	return nil
}

// ImportSupportCenters loads support-centers.yaml into the support_centers table.
func ImportSupportCenters(ctx context.Context, q *db.Queries, scs map[string]SupportCenterYAML) error {
	for name, sc := range scs {
		id := sc.ID
		if id == 0 {
			id = GenID(name)
		}
		if err := q.UpsertSupportCenter(ctx, id, name, sc.LongName, sc.Community, sc.Description); err != nil {
			return fmt.Errorf("upsert support center %q: %w", name, err)
		}
	}
	return nil
}

// ImportTree reads a full topology root (tree + flat files) and imports it.
func ImportTree(ctx context.Context, q *db.Queries, root string) error {
	if svcs, err := ReadServices(root); err == nil {
		if err := ImportServices(ctx, q, svcs); err != nil {
			return err
		}
	}
	if scs, err := ReadSupportCenters(root); err == nil {
		if err := ImportSupportCenters(ctx, q, scs); err != nil {
			return err
		}
	}
	tree, err := ReadTree(root)
	if err != nil {
		return err
	}
	return Import(ctx, q, tree)
}

// UpsertResource inserts a resource (with its services and contact lists) under
// the given resource-group id. Reused by both the bulk importer and the
// change-proposal apply path so their behavior stays identical. A nil res.ID
// falls back to gen_id (id_explicit=false), the correct behavior for a new
// registration.
func UpsertResource(ctx context.Context, q *db.Queries, rgID, resName string, res *Resource) (string, error) {
	topID, explicit := resolveID(res.ID, resName)
	resID, err := q.InsertResource(ctx, db.ResourceRow{
		TopologyID: topID, ResourceGroupID: rgID, Name: resName,
		Active: res.Active, Description: res.Description, FQDN: res.FQDN,
		DN: res.DN, FQDNAliases: res.FQDNAliases, Tags: res.Tags,
		AllowedVOs: res.AllowedVOs, VOOwnership: mustJSONAny(res.VOOwnership),
		WLCGInformation: mustJSONAny(res.WLCGInformation), Extra: mustJSON(res.Extra),
		IDExplicit: explicit,
	})
	if err != nil {
		return "", fmt.Errorf("insert resource %q: %w", resName, err)
	}
	ord := 0
	for svcName, svc := range res.Services {
		if err := q.InsertResourceService(ctx, db.ResourceServiceRow{
			ResourceID: resID, ServiceName: svcName, Description: svc.Description,
			Details: mustJSONAny(svcBlob{Details: svc.Details, Extra: svc.Extra}), Ordinal: ord,
		}); err != nil {
			return "", fmt.Errorf("insert service %q: %w", svcName, err)
		}
		ord++
	}
	for ctype, ranks := range res.ContactLists {
		for rank, contact := range ranks {
			if err := q.InsertResourceContact(ctx, db.ResourceContactRow{
				ResourceID: resID, ContactType: ctype, Rank: rank,
				ContactName: contact.Name, ContactID: contact.ID,
			}); err != nil {
				return "", fmt.Errorf("insert contact: %w", err)
			}
		}
	}
	return resID, nil
}

// ExportFullToDir writes the entire topology domain (tree + services.yaml +
// support-centers.yaml) to a directory, the on-disk form used for backups.
func ExportFullToDir(ctx context.Context, q *db.Queries, root string) error {
	tree, err := Export(ctx, q)
	if err != nil {
		return err
	}
	if err := WriteTree(root, tree); err != nil {
		return err
	}
	// services.yaml
	svcs, err := q.ListAllServices(ctx)
	if err != nil {
		return err
	}
	if len(svcs) > 0 {
		if err := writeYAMLFile(filepath.Join(root, "services.yaml"), svcs); err != nil {
			return err
		}
	}
	// support-centers.yaml
	scs, err := q.ListAllSupportCenters(ctx)
	if err != nil {
		return err
	}
	if len(scs) > 0 {
		m := map[string]SupportCenterYAML{}
		for _, s := range scs {
			m[s.Name] = SupportCenterYAML{
				ID: s.ID, LongName: s.LongName, Community: s.Community, Description: s.Description,
			}
		}
		if err := writeYAMLFile(filepath.Join(root, "support-centers.yaml"), m); err != nil {
			return err
		}
	}
	return nil
}

// Export reads the whole topology domain out of the database into a Topology,
// reconstructing the directory parentage.
func Export(ctx context.Context, q *db.Queries) (*Topology, error) {
	t := &Topology{
		Facilities:     map[string]*Facility{},
		Sites:          map[string]*Site{},
		ResourceGroups: map[string]*ResourceGroup{},
		Downtimes:      map[string][]*Downtime{},
		SiteFacility:   map[string]string{},
		RGSite:         map[string]string{},
	}

	facs, err := q.ListFacilities(ctx)
	if err != nil {
		return nil, err
	}
	for _, f := range facs {
		t.Facilities[f.Name] = &Facility{
			Name: f.Name, ID: idPtr(f.TopologyID, f.IDExplicit),
			InstitutionID: f.InstitutionID, Extra: fromJSON(f.Extra),
		}
	}

	sites, err := q.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range sites {
		t.Sites[s.Name] = &Site{
			Name: s.Name, ID: idPtr(s.TopologyID, s.IDExplicit), LongName: s.LongName,
			Description: s.Description, AddressLine1: s.AddressLine1, AddressLine2: s.AddressLine2,
			City: s.City, State: s.State, Country: s.Country, Zipcode: s.Zipcode,
			Latitude: s.Latitude, Longitude: s.Longitude, Extra: fromJSON(s.Extra),
		}
		t.SiteFacility[s.Name] = s.FacilityName
	}

	rgs, err := q.ListResourceGroups(ctx)
	if err != nil {
		return nil, err
	}
	rgByID := map[string]*ResourceGroup{}
	for _, rg := range rgs {
		g := &ResourceGroup{
			Name: rg.Name, GroupID: idPtr(rg.GroupID, rg.IDExplicit), Production: rg.Production,
			SupportCenter: rg.SupportCenter, GroupDescription: rg.GroupDescription,
			Resources: map[string]*Resource{}, Extra: fromJSON(rg.Extra),
		}
		t.ResourceGroups[rg.Name] = g
		t.RGSite[rg.Name] = rg.SiteName
		rgByID[rg.ID] = g
	}

	resources, err := q.ListResources(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range resources {
		res := &Resource{
			ID: idPtr(r.TopologyID, r.IDExplicit), Active: r.Active, Description: r.Description,
			FQDN: r.FQDN, DN: r.DN, FQDNAliases: r.FQDNAliases, Tags: r.Tags,
			AllowedVOs: r.AllowedVOs, VOOwnership: fromJSONAny(r.VOOwnership),
			WLCGInformation: fromJSONAny(r.WLCGInformation), Extra: fromJSON(r.Extra),
		}
		// Services.
		svcs, err := q.ListResourceServices(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		if len(svcs) > 0 {
			res.Services = map[string]*Service{}
			for _, s := range svcs {
				res.Services[s.ServiceName] = serviceFromBlob(s.Description, s.Details)
			}
		}
		// Contacts.
		contacts, err := q.ListResourceContacts(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		if len(contacts) > 0 {
			res.ContactLists = map[string]map[string]Contact{}
			for _, c := range contacts {
				if res.ContactLists[c.ContactType] == nil {
					res.ContactLists[c.ContactType] = map[string]Contact{}
				}
				res.ContactLists[c.ContactType][c.Rank] = Contact{Name: c.ContactName, ID: c.ContactID}
			}
		}
		if g := rgByID[r.ResourceGroupID]; g != nil {
			g.Resources[r.Name] = res
		} else {
			// RGName carried the join; fall back to it.
			if gg := t.ResourceGroups[r.RGName]; gg != nil {
				gg.Resources[r.Name] = res
			}
		}
	}

	dts, err := q.ListDowntimes(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range dts {
		t.Downtimes[d.RGName] = append(t.Downtimes[d.RGName], &Downtime{
			ID: d.DtID, Class: d.Class, Description: d.Description, Severity: d.Severity,
			StartTime: d.StartTime, EndTime: d.EndTime, CreatedTime: d.CreatedTime,
			ResourceName: d.ResourceName, Services: d.Services,
		})
	}
	return t, nil
}

// ---- helpers ----

// resolveID returns the topology id and whether it was explicit in the source.
func resolveID(id *int64, name string) (int64, bool) {
	if id != nil {
		return *id, true
	}
	return GenID(name), false
}

func idPtr(id int64, explicit bool) *int64 {
	if !explicit {
		return nil
	}
	v := id
	return &v
}

func mustJSON(v map[string]interface{}) []byte {
	if len(v) == 0 {
		return nil
	}
	return mustJSONAny(v)
}

// mustJSONAny marshals any value (map, scalar, slice) to JSON, or nil if empty.
func mustJSONAny(v interface{}) []byte {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

func fromJSON(b []byte) map[string]interface{} {
	if len(b) == 0 {
		return nil
	}
	var v map[string]interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil
	}
	return v
}

// fromJSONAny unmarshals JSON into an arbitrary value (map, scalar, slice).
func fromJSONAny(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return nil
	}
	return v
}

// svcBlob is the JSON storage form of a Service (Description is stored in its
// own column). Details is polymorphic; Extra holds any inline unmodeled keys.
type svcBlob struct {
	Details interface{}            `json:"details,omitempty"`
	Extra   map[string]interface{} `json:"extra,omitempty"`
}

// serviceFromBlob reconstructs a Service from its stored description and blob.
func serviceFromBlob(desc string, blob []byte) *Service {
	svc := &Service{Description: desc}
	if len(blob) == 0 {
		return svc
	}
	var b svcBlob
	if err := json.Unmarshal(blob, &b); err != nil {
		return svc
	}
	svc.Details = b.Details
	svc.Extra = b.Extra
	return svc
}
