package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
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
// Contact ids are PII: only a contact_reader (or manager/admin) sees them;
// everyone else gets names only.
func (h *Handler) ContactsHandler(w http.ResponseWriter, r *http.Request) {
	contacts, err := h.queries.ListDistinctContacts(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !models.HasContactReader(rolesFromContext(r.Context())) {
		for i := range contacts {
			contacts[i].ID = ""
		}
	}
	respondJSON(w, http.StatusOK, contacts)
}

// SearchContactableUsersHandler is a non-admin-safe real-user search, so any
// authenticated user (not just admins) can find and pick a real person as a
// contact -- unlike ContactsHandler above, which only ever surfaces names
// already used somewhere in resource_contacts, not real users table rows.
// Returns legacy_contact_id as "id" -- the one identifier a contact has (see
// EntityContact's doc comment in internal/db/queries_contacts.go), not the
// users.id surrogate key. A user with none yet (no email ever captured) is
// excluded -- there's nothing valid to pick them by.
func (h *Handler) SearchContactableUsersHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	users, err := h.queries.SearchUsers(r.Context(), q, 25)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		if u.Status != "active" || u.LegacyContactID == "" {
			continue
		}
		out = append(out, map[string]any{"id": u.LegacyContactID, "display_name": u.DisplayName})
	}
	respondJSON(w, http.StatusOK, out)
}

// DowntimesHandler returns downtimes, optionally filtered by ?resource= or ?rg=.
func (h *Handler) DowntimesHandler(w http.ResponseWriter, r *http.Request) {
	all, err := h.queries.ListDowntimes(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resFilter := r.URL.Query().Get("resource")
	rgFilter := r.URL.Query().Get("rg")
	out := make([]map[string]any, 0)
	for _, d := range all {
		if resFilter != "" && d.ResourceName != resFilter {
			continue
		}
		if rgFilter != "" && d.RGName != rgFilter {
			continue
		}
		out = append(out, downtimeJSON(d))
	}
	respondJSON(w, http.StatusOK, out)
}

func downtimeJSON(d db.DowntimeRow) map[string]any {
	return map[string]any{
		"id": d.DtID, "resource_group": d.RGName, "resource": d.ResourceName,
		"class": d.Class, "severity": d.Severity, "description": d.Description,
		"start_time": d.StartTime, "end_time": d.EndTime, "created_time": d.CreatedTime,
		"services": strOrEmpty(d.Services),
	}
}

// UserLabelsHandler returns display labels ("Display name (username)") for a
// comma-separated set of user ids, so actors can be shown wherever a change was
// made by someone.
func (h *Handler) UserLabelsHandler(w http.ResponseWriter, r *http.Request) {
	idsParam := r.URL.Query().Get("ids")
	if idsParam == "" {
		respondJSON(w, http.StatusOK, []any{})
		return
	}
	ids := strings.Split(idsParam, ",")
	labels, err := h.queries.UserLabels(r.Context(), ids)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, labels)
}

// ListInstitutionsHandler returns cached institutions, optionally filtered by a
// ?q= name substring (used by the facility institution picker).
func (h *Handler) ListInstitutionsHandler(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var (
		rows any
		err  error
	)
	if q != "" {
		rows, err = h.queries.SearchInstitutions(r.Context(), q, 50)
	} else {
		rows, err = h.queries.ListInstitutions(r.Context())
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, rows)
}

// lastInstitutionRefresh throttles user-triggered registry refreshes.
var (
	lastInstitutionRefresh   time.Time
	institutionRefreshMu     sync.Mutex
	institutionRefreshWindow = 5 * time.Minute
)

// RefreshInstitutionsHandler lets any authenticated user pull newly-registered
// institutions from the registry, rate-limited so a missing-name lookup can't
// hammer the upstream. Soft failure: the cache stays authoritative.
func (h *Handler) RefreshInstitutionsHandler(w http.ResponseWriter, r *http.Request) {
	institutionRefreshMu.Lock()
	since := time.Since(lastInstitutionRefresh)
	if since < institutionRefreshWindow {
		wait := int((institutionRefreshWindow - since).Seconds())
		institutionRefreshMu.Unlock()
		respondJSON(w, http.StatusOK, map[string]any{"throttled": true, "retry_after_seconds": wait})
		return
	}
	lastInstitutionRefresh = time.Now()
	institutionRefreshMu.Unlock()

	n, err := h.syncInstitutions(r.Context())
	if err != nil {
		respondError(w, http.StatusBadGateway, "institutions registry unavailable: "+err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"synced": n})
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
