package handlers

import (
	"encoding/xml"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/xmlapi"
)

// parseFilters builds an xmlapi.Filters from legacy-style query args:
//
//	facility_<id>=on | facility_sel[]=<id>   (also rg_, site_, sc_, service_)
//	gridtype=on&gridtype_1=on&gridtype_2=on
//	active=on&active_value=1|0
func parseFilters(r *http.Request) xmlapi.Filters {
	q := r.URL.Query()
	f := xmlapi.Filters{
		FacilityIDs: map[int64]bool{},
		SiteIDs:     map[int64]bool{},
		RGIDs:       map[int64]bool{},
		SCIDs:       map[int64]bool{},
		ServiceIDs:  map[int64]bool{},
	}
	add := func(m map[int64]bool, v string) {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			m[id] = true
		}
	}
	for key, vals := range q {
		switch {
		case strings.HasPrefix(key, "facility_sel"):
			for _, v := range vals {
				add(f.FacilityIDs, v)
			}
		case strings.HasPrefix(key, "facility_"):
			add(f.FacilityIDs, strings.TrimPrefix(key, "facility_"))
		case strings.HasPrefix(key, "site_sel"):
			for _, v := range vals {
				add(f.SiteIDs, v)
			}
		case strings.HasPrefix(key, "site_"):
			add(f.SiteIDs, strings.TrimPrefix(key, "site_"))
		case strings.HasPrefix(key, "rg_sel"):
			for _, v := range vals {
				add(f.RGIDs, v)
			}
		case strings.HasPrefix(key, "rg_"):
			add(f.RGIDs, strings.TrimPrefix(key, "rg_"))
		case strings.HasPrefix(key, "sc_sel"):
			for _, v := range vals {
				add(f.SCIDs, v)
			}
		case strings.HasPrefix(key, "sc_"):
			add(f.SCIDs, strings.TrimPrefix(key, "sc_"))
		case strings.HasPrefix(key, "service_sel"):
			for _, v := range vals {
				add(f.ServiceIDs, v)
			}
		case strings.HasPrefix(key, "service_"):
			add(f.ServiceIDs, strings.TrimPrefix(key, "service_"))
		}
	}
	if q.Get("gridtype") == "on" {
		f.GridTypeProd = q.Get("gridtype_1") == "on"
		f.GridTypeITB = q.Get("gridtype_2") == "on"
	}
	if q.Get("active") == "on" {
		v := q.Get("active_value") == "1"
		f.Active = &v
	}
	return f
}

// includePII reports whether contact PII (emails, ids) may be exposed. Only a
// session holding the contact_reader capability (or manager/administrator)
// unlocks it; everyone else — including anonymous clients — sees names only.
func (h *Handler) includePII(r *http.Request) bool {
	return models.HasContactReader(rolesFromContext(r.Context()))
}

// RGSummaryXML serves /rgsummary/xml.
func (h *Handler) RGSummaryXML(w http.ResponseWriter, r *http.Request) {
	summary, err := xmlapi.BuildResourceSummary(r.Context(), h.queries, h.encryptor, parseFilters(r), h.includePII(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeXML(w, summary)
}

// RGSummaryJSON serves /api/v1/rgsummary (JSON form of the same data).
func (h *Handler) RGSummaryJSON(w http.ResponseWriter, r *http.Request) {
	summary, err := xmlapi.BuildResourceSummary(r.Context(), h.queries, h.encryptor, parseFilters(r), h.includePII(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, summary)
}

// RGDowntimeXML serves /rgdowntime/xml.
func (h *Handler) RGDowntimeXML(w http.ResponseWriter, r *http.Request) {
	dts, err := xmlapi.BuildDowntimes(r.Context(), h.queries, parseFilters(r), time.Now())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeXML(w, dts)
}

// MiscResourceJSON serves a resource-keyed JSON summary in the legacy
// (capitalized-key) shape for /miscresource/json compatibility.
func (h *Handler) MiscResourceJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListResources(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{}
	for _, res := range rows {
		out[res.Name] = map[string]any{
			"ID": res.TopologyID, "Name": res.Name, "FQDN": res.FQDN,
			"Active": res.Active, "ResourceGroup": res.RGName,
		}
	}
	respondJSON(w, http.StatusOK, out)
}

// ResourcesJSON serves the resource list in the frontend's snake_case shape
// (matching /dashboard's my_resources), keyed by resource name.
func (h *Handler) ResourcesJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListResources(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{}
	for _, res := range rows {
		active := res.Active != nil && *res.Active
		out[res.Name] = map[string]any{
			"id": res.TopologyID, "name": res.Name, "fqdn": res.FQDN,
			"active": active, "resource_group": res.RGName,
		}
	}
	respondJSON(w, http.StatusOK, out)
}

// MiscSiteJSON serves a site-keyed JSON summary.
func (h *Handler) MiscSiteJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListSites(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{}
	for _, s := range rows {
		out[s.Name] = map[string]any{
			"ID": s.TopologyID, "Name": s.Name, "Facility": s.FacilityName,
			"City": s.City, "Country": s.Country,
		}
	}
	respondJSON(w, http.StatusOK, out)
}

// MiscFacilityJSON serves a facility-keyed JSON summary.
func (h *Handler) MiscFacilityJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListFacilities(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{}
	for _, fac := range rows {
		out[fac.Name] = map[string]any{
			"ID": fac.TopologyID, "Name": fac.Name, "InstitutionID": fac.InstitutionID,
		}
	}
	respondJSON(w, http.StatusOK, out)
}

// VOSummaryXML serves /vosummary/xml.
func (h *Handler) VOSummaryXML(w http.ResponseWriter, r *http.Request) {
	summary, err := xmlapi.BuildVOSummary(r.Context(), h.queries)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeXML(w, summary)
}

// MiscProjectXML serves /miscproject/xml.
func (h *Handler) MiscProjectXML(w http.ResponseWriter, r *http.Request) {
	projects, err := xmlapi.BuildProjects(r.Context(), h.queries)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeXML(w, projects)
}

// VOSummaryJSON serves virtual organizations keyed by name (parsed from the
// stored document form).
func (h *Handler) VOSummaryJSON(w http.ResponseWriter, r *http.Request) {
	vos, err := h.queries.ListVOs(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{}
	for _, v := range vos {
		var doc map[string]any
		if err := yaml.Unmarshal(v.Raw, &doc); err == nil {
			out[v.Name] = doc
		}
	}
	respondJSON(w, http.StatusOK, out)
}

// MiscProjectJSON serves projects keyed by name, reconstructed from the
// relational columns (typed fields + sponsor + any extra).
func (h *Handler) MiscProjectJSON(w http.ResponseWriter, r *http.Request) {
	projects, err := h.queries.ListProjects(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{}
	for _, p := range projects {
		m := map[string]any{
			"Name": p.Name, "ID": p.ProjectID, "Description": p.Description,
			"Department": p.Department, "FieldOfScience": p.FieldOfScience,
			"FieldOfScienceID": p.FieldOfScienceID, "Organization": p.Organization,
			"PIName": p.PIName, "InstitutionID": p.InstitutionID,
		}
		if len(p.Sponsor) > 0 {
			var s any
			_ = yaml.Unmarshal(p.Sponsor, &s)
			m["Sponsor"] = s
		}
		if len(p.Extra) > 0 {
			var e map[string]any
			if yaml.Unmarshal(p.Extra, &e) == nil {
				for k, v := range e {
					m[k] = v
				}
			}
		}
		out[p.Name] = m
	}
	respondJSON(w, http.StatusOK, out)
}

// ServeSchema serves the embedded XSD files at /schema/<file>.
func (h *Handler) ServeSchema(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "file")
	data, err := fs.ReadFile(xmlapi.SchemaFiles(), name)
	if err != nil {
		respondError(w, http.StatusNotFound, "schema not found")
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write(data)
}

// writeXML marshals v with the XML declaration and text/xml content type.
func writeXML(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(v); err != nil {
		respondError(w, http.StatusInternalServerError, "encoding xml")
	}
}
