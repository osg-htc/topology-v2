package handlers

import (
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

	// Contacts grouped by type → rank.
	contactRows, _ := h.queries.ListResourceContacts(ctx, resID)
	contacts := make([]map[string]any, 0, len(contactRows))
	for _, c := range contactRows {
		contacts = append(contacts, map[string]any{
			"contact_type": c.ContactType, "rank": c.Rank,
			"name": c.ContactName, "id": c.ContactID,
		})
	}

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
			respondJSON(w, http.StatusOK, map[string]any{
				"name": rg.Name, "group_id": rg.GroupID, "site": rg.Site,
				"facility": rg.Facility, "production": rg.Production,
				"support_center": rg.SupportCenter, "group_description": rg.GroupDescription,
				"deleted": rg.Deleted, "resources": strOrEmpty(resources),
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
	out := map[string]any{
		"name": s.Name, "facility": facName, "long_name": s.LongName,
		"description": s.Description, "address_line1": s.AddressLine1,
		"address_line2": s.AddressLine2, "city": s.City, "state": s.State,
		"country": s.Country, "zipcode": s.Zipcode,
		"resource_groups": strOrEmpty(rgs),
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
			respondJSON(w, http.StatusOK, map[string]any{
				"name": f.Name, "facility_id": f.FacilityID,
				"institution_id": f.InstitutionID, "deleted": f.Deleted,
				"sites": strOrEmpty(sites),
			})
			return
		}
	}
	respondError(w, http.StatusNotFound, "facility not found")
}

func strOrEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
