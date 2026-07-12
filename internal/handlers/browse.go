package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

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
