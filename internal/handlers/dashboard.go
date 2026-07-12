package handlers

import (
	"context"
	"net/http"

	"github.com/bbockelm/topology-v2/internal/models"
)

// DashboardHandler returns the resource-centric home view: the resources the
// user owns (is a contact for), their pending registrations, and — for
// managers/admins — the pending-approval queue.
func (h *Handler) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := h.currentUser(r)

	legacyIDs := h.legacyContactIDs(ctx, u.ID)
	myResources, err := h.queries.ResourcesForContactIDs(ctx, u.ID, legacyIDs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "loading resources")
		return
	}

	proposals, err := h.queries.ListProposalsByCreator(ctx, u.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "loading proposals")
		return
	}
	var pendingRegs []*models.Proposal
	for _, p := range proposals {
		if p.Operation == models.OpCreate &&
			(p.Status == models.ProposalDraft || p.Status == models.ProposalPending) {
			pendingRegs = append(pendingRegs, p)
		}
	}

	var pendingApprovals []*models.Proposal
	if h.isManagerOrAdmin(r) {
		if ps, err := h.queries.ListPendingProposals(ctx); err == nil {
			pendingApprovals = ps
		}
	}

	resources := make([]map[string]any, 0, len(myResources))
	for _, res := range myResources {
		resources = append(resources, map[string]any{
			"name": res.Name, "fqdn": res.FQDN, "active": res.Active,
			"resource_group": res.RGName, "id": res.TopologyID,
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"my_resources":          resources,
		"pending_registrations": pendingRegs,
		"pending_approvals":     pendingApprovals,
		"can_review":            h.isManagerOrAdmin(r),
	})
}

// legacyContactIDs gathers the legacy contact ids (CILogon id and email SHA-1)
// across all of a user's identities, used to match resource_contacts entries.
func (h *Handler) legacyContactIDs(ctx context.Context, userID string) []string {
	rows, err := h.queries.ListUserIdentities(ctx, userID)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var ids []string
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			ids = append(ids, s)
		}
	}
	for _, row := range rows {
		id := row.Identity()
		add(id.CILogonID)
		add(id.EmailSHA1)
	}
	return ids
}

// primaryCILogonID returns the user's first CILogon id across their identities.
func (h *Handler) primaryCILogonID(ctx context.Context, userID string) string {
	rows, err := h.queries.ListUserIdentities(ctx, userID)
	if err != nil {
		return ""
	}
	for _, row := range rows {
		if id := row.Identity().CILogonID; id != "" {
			return id
		}
	}
	return ""
}
