package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ResourceDetailHandler returns a single resource with services, contacts, tags,
// VO ownership, and parentage — the full picture for the detail page.
func (h *Handler) ResourceDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := chi.URLParam(r, "name")
	d, err := h.queries.GetResourceDetail(ctx, name)
	if err != nil {
		respondError(w, http.StatusNotFound, "resource not found")
		return
	}
	resID, _ := h.queries.ResourceIDByName(ctx, name)

	// Services.
	svcRows, _ := h.queries.ListResourceServices(ctx, resID)
	services := make([]map[string]any, 0, len(svcRows))
	for _, s := range svcRows {
		m := map[string]any{"name": s.ServiceName, "description": s.Description}
		if len(s.Details) > 0 {
			var blob map[string]any
			if json.Unmarshal(s.Details, &blob) == nil {
				if blob["details"] != nil {
					m["details"] = blob["details"]
				}
			}
		}
		services = append(services, m)
	}

	// Effective contacts: the resource's own contacts, plus inherited contacts
	// from its resource group → site → facility for any contact type the
	// resource doesn't define itself. Each entry carries inherited_from.
	contacts := h.effectiveResourceContacts(ctx, resID, d.ResourceGroup, d.Site, d.Facility)

	active := d.Active != nil && *d.Active
	out := map[string]any{
		"name": d.Name, "id": d.TopologyID, "resource_group": d.ResourceGroup,
		"site": d.Site, "facility": d.Facility, "active": active,
		"description": d.Description, "fqdn": d.FQDN, "dn": d.DN,
		"fqdn_aliases": strOrEmpty(d.FQDNAliases), "tags": strOrEmpty(d.Tags),
		"allowed_vos": strOrEmpty(d.AllowedVOs), "services": services, "contacts": contacts,
	}
	if len(d.VOOwnership) > 0 {
		var vo any
		if json.Unmarshal(d.VOOwnership, &vo) == nil {
			out["vo_ownership"] = vo
		}
	}
	if len(d.WLCGInformation) > 0 {
		var wl any
		if json.Unmarshal(d.WLCGInformation, &wl) == nil {
			out["wlcg_information"] = wl
		}
	}

	// Downtimes affecting this resource.
	dts, _ := h.queries.ListDowntimes(ctx)
	downtimes := make([]map[string]any, 0)
	for _, dt := range dts {
		if dt.ResourceName == name {
			downtimes = append(downtimes, downtimeJSON(dt))
		}
	}
	out["downtimes"] = downtimes

	respondJSON(w, http.StatusOK, out)
}

// ResourceGroupDetailHandler returns an RG with its resources.
func (h *Handler) ResourceGroupDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := chi.URLParam(r, "name")
	rgs, err := h.queries.ListBrowseRGs(ctx, true)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, rg := range rgs {
		if rg.Name == name {
			resources, _ := h.queries.ResourceNamesInRG(ctx, name)
			contacts, _ := h.queries.ListEntityContacts(ctx, "resource_group", name)
			respondJSON(w, http.StatusOK, map[string]any{
				"name": rg.Name, "group_id": rg.GroupID, "site": rg.Site,
				"facility": rg.Facility, "production": rg.Production,
				"support_center": rg.SupportCenter, "group_description": rg.GroupDescription,
				"deleted": rg.Deleted, "resources": strOrEmpty(resources), "contacts": contacts,
			})
			return
		}
	}
	respondError(w, http.StatusNotFound, "resource group not found")
}

// SiteDetailHandler returns a site with its resource groups.
func (h *Handler) SiteDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := chi.URLParam(r, "name")
	s, facName, err := h.queries.GetSiteDetail(ctx, name)
	if err != nil {
		respondError(w, http.StatusNotFound, "site not found")
		return
	}
	rgs, _ := h.queries.RGNamesInSite(ctx, name)
	contacts, _ := h.queries.ListEntityContacts(ctx, "site", name)
	out := map[string]any{
		"name": s.Name, "facility": facName, "long_name": s.LongName,
		"description": s.Description, "address_line1": s.AddressLine1,
		"address_line2": s.AddressLine2, "city": s.City, "state": s.State,
		"country": s.Country, "zipcode": s.Zipcode,
		"resource_groups": strOrEmpty(rgs), "contacts": contacts,
	}
	if s.Latitude != nil {
		out["latitude"] = *s.Latitude
	}
	if s.Longitude != nil {
		out["longitude"] = *s.Longitude
	}
	respondJSON(w, http.StatusOK, out)
}

// FacilityDetailHandler returns a facility with its sites.
func (h *Handler) FacilityDetailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := chi.URLParam(r, "name")
	facs, err := h.queries.ListBrowseFacilities(ctx, true)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, f := range facs {
		if f.Name == name {
			sites, _ := h.queries.SiteNamesInFacility(ctx, name)
			contacts, _ := h.queries.ListEntityContacts(ctx, "facility", name)
			respondJSON(w, http.StatusOK, map[string]any{
				"name": f.Name, "facility_id": f.FacilityID,
				"institution_id": f.InstitutionID, "deleted": f.Deleted,
				"sites": strOrEmpty(sites), "contacts": contacts,
			})
			return
		}
	}
	respondError(w, http.StatusNotFound, "facility not found")
}

// effectiveResourceContacts merges a resource's own contacts with contacts
// inherited from its RG → site → facility. A contact type defined on the
// resource fully overrides the same type from any ancestor; otherwise the
// nearest ancestor that defines the type is inherited.
func (h *Handler) effectiveResourceContacts(ctx context.Context, resID, rg, site, facility string) []map[string]any {
	out := make([]map[string]any, 0)
	covered := map[string]bool{} // contact types already resolved

	own, _ := h.queries.ListResourceContacts(ctx, resID)
	for _, c := range own {
		covered[c.ContactType] = true
		out = append(out, map[string]any{
			"contact_type": c.ContactType, "rank": c.Rank,
			"name": c.ContactName, "id": c.ContactID, "inherited_from": "",
		})
	}

	// Walk ancestors nearest-first; add a type only if not already covered.
	ancestors := []struct{ kind, name, label string }{
		{"resource_group", rg, "resource group"},
		{"site", site, "site"},
		{"facility", facility, "facility"},
	}
	for _, a := range ancestors {
		if a.name == "" {
			continue
		}
		ec, _ := h.queries.ListEntityContacts(ctx, a.kind, a.name)
		// Which types does this level introduce (not yet covered)?
		introduced := map[string]bool{}
		for _, c := range ec {
			if covered[c.ContactType] {
				continue
			}
			introduced[c.ContactType] = true
			out = append(out, map[string]any{
				"contact_type": c.ContactType, "rank": c.Rank,
				"name": c.Name, "id": c.ID, "inherited_from": a.label,
			})
		}
		for t := range introduced {
			covered[t] = true
		}
	}
	return out
}

func strOrEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
