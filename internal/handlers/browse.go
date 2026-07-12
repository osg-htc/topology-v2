package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// includeInactive reports whether ?include_inactive is truthy.
func includeInactive(r *http.Request) bool {
	v := r.URL.Query().Get("include_inactive")
	return v == "1" || v == "true"
}

// SummaryHandler returns active-entity counts for the dashboard.
func (h *Handler) SummaryHandler(w http.ResponseWriter, r *http.Request) {
	s, err := h.queries.CountsSummary(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, s)
}

// ListResourceGroupsHandler lists resource groups for the management UI.
func (h *Handler) ListResourceGroupsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListBrowseRGs(r.Context(), includeInactive(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// ListSitesHandler lists sites for the management UI.
func (h *Handler) ListSitesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListBrowseSites(r.Context(), includeInactive(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// ListFacilitiesHandler lists facilities for the management UI.
func (h *Handler) ListFacilitiesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListBrowseFacilities(r.Context(), includeInactive(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// ListProjectsBrowseHandler lists projects for the management UI.
func (h *Handler) ListProjectsBrowseHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListBrowseProjects(r.Context(), includeInactive(r))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// GetProjectHandler returns one project's full detail.
func (h *Handler) GetProjectHandler(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	p, err := h.queries.GetProjectByName(r.Context(), name)
	if err != nil {
		respondError(w, http.StatusNotFound, "project not found")
		return
	}
	out := map[string]any{
		"name": p.Name, "id": p.ProjectID, "description": p.Description,
		"department": p.Department, "field_of_science": p.FieldOfScience,
		"field_of_science_id": p.FieldOfScienceID, "organization": p.Organization,
		"pi_name": p.PIName, "institution_id": p.InstitutionID,
		"sponsor_type": p.SponsorType, "sponsor_name": p.SponsorName,
	}
	if len(p.Sponsor) > 0 {
		var s any
		_ = json.Unmarshal(p.Sponsor, &s)
		out["sponsor"] = s
	}
	respondJSON(w, http.StatusOK, out)
}

// ServiceNamesHandler returns known service names (for the service dropdown).
func (h *Handler) ServiceNamesHandler(w http.ResponseWriter, r *http.Request) {
	names, err := h.queries.ListServiceNames(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, names)
}

// VONamesHandler returns active VO names (for the allowed-VOs multiselect).
func (h *Handler) VONamesHandler(w http.ResponseWriter, r *http.Request) {
	names, err := h.queries.ListVONames(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, names)
}

// TagsHandler returns distinct resource tags in use (for the tags multiselect).
func (h *Handler) TagsHandler(w http.ResponseWriter, r *http.Request) {
	tags, err := h.queries.ListDistinctTags(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, tags)
}

// ContactsHandler returns distinct known contacts (for the contact picker).
func (h *Handler) ContactsHandler(w http.ResponseWriter, r *http.Request) {
	contacts, err := h.queries.ListDistinctContacts(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, contacts)
}

// ListInstitutionsHandler returns cached institutions.
func (h *Handler) ListInstitutionsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := h.queries.ListInstitutions(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// SyncInstitutionsHandler refreshes the institution cache from the external
// registry. Administrator only. Failure is soft (cache remains authoritative).
func (h *Handler) SyncInstitutionsHandler(w http.ResponseWriter, r *http.Request) {
	n, err := h.syncInstitutions(r.Context())
	if err != nil {
		respondError(w, http.StatusBadGateway, "institutions registry unavailable: "+err.Error())
		return
	}
	h.audit(r.Context(), h.currentUser(r).ID, "institutions.sync", "institution", "", "", nil)
	respondJSON(w, http.StatusOK, map[string]int{"synced": n})
}

type institutionAPIEntry struct {
	Name  string `json:"name"`
	ID    string `json:"id"`
	RORID string `json:"ror_id"`
}

// syncInstitutions fetches the registry and upserts each institution.
func (h *Handler) syncInstitutions(ctx context.Context) (int, error) {
	url := h.cfg.InstitutionsAPI + "/institution_ids"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, errStatus(resp.StatusCode)
	}
	var entries []institutionAPIEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		if e.ID == "" {
			continue
		}
		if err := h.queries.UpsertInstitution(ctx, e.ID, e.Name, e.RORID); err != nil {
			log.Error().Err(err).Str("iid", e.ID).Msg("caching institution")
			continue
		}
		n++
	}
	return n, nil
}

type statusError int

func (e statusError) Error() string { return "unexpected status " + http.StatusText(int(e)) }
func errStatus(code int) error      { return statusError(code) }
