package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/bbockelm/topology-v2/internal/db"
)

type createReplacementRequest struct {
	EntityKind  string `json:"entity_kind"`
	EntityName  string `json:"entity_name"`
	ContactType string `json:"contact_type"`
	Rank        string `json:"rank"`
	Note        string `json:"note"`
	// RequesterUserID lets an invite-acceptance flow file the request on behalf
	// of the accepting user; ignored for direct calls (the session user is used).
	RequesterUserID string `json:"-"`
}

var replacementKinds = map[string]bool{
	"resource": true, "resource_group": true, "site": true, "facility": true,
}

// CreateContactReplacement files a request for the current user to take over a
// contact slot from its present holder.
func (h *Handler) CreateContactReplacement(w http.ResponseWriter, r *http.Request) {
	var req createReplacementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	req.RequesterUserID = h.currentUser(r).ID
	id, err := h.fileReplacement(r, req)
	if err != nil {
		respondError(w, err.status, err.msg)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "pending"})
}

type httpErr struct {
	status int
	msg    string
}

func (e *httpErr) Error() string { return e.msg }

// fileReplacement resolves the incumbent slot and inserts a pending request.
// Shared by the direct endpoint and invite acceptance.
func (h *Handler) fileReplacement(r *http.Request, req createReplacementRequest) (string, *httpErr) {
	ctx := r.Context()
	if !replacementKinds[req.EntityKind] || req.EntityName == "" || req.ContactType == "" || req.Rank == "" {
		return "", &httpErr{http.StatusBadRequest, "entity_kind, entity_name, contact_type and rank are required"}
	}
	slot, _ := h.queries.GetContactSlot(ctx, req.EntityKind, req.EntityName, req.ContactType, req.Rank)
	if !slot.Found {
		return "", &httpErr{http.StatusNotFound, "no such contact slot to replace"}
	}
	if slot.UserID != "" && slot.UserID == req.RequesterUserID {
		return "", &httpErr{http.StatusBadRequest, "you already hold this contact slot"}
	}
	requester, err := h.queries.GetUser(ctx, req.RequesterUserID)
	if err != nil {
		return "", &httpErr{http.StatusInternalServerError, "loading requester"}
	}
	id, err := h.queries.CreateContactReplacement(ctx, db.CreateContactReplacementParams{
		EntityKind: req.EntityKind, EntityName: req.EntityName, ContactType: req.ContactType, Rank: req.Rank,
		IncumbentUserID: slot.UserID, IncumbentName: slot.Name,
		RequesterUserID: req.RequesterUserID, RequesterName: requester.DisplayName,
		RequesterContactID: h.primaryCILogonID(ctx, req.RequesterUserID), Note: req.Note,
	})
	if err != nil {
		return "", &httpErr{http.StatusInternalServerError, "creating request"}
	}
	h.audit(ctx, req.RequesterUserID, "replacement.create", req.EntityKind, req.EntityName, id, nil)
	return id, nil
}

// ListMyReplacements returns replacement requests the current user has filed.
func (h *Handler) ListMyReplacements(w http.ResponseWriter, r *http.Request) {
	rs, err := h.queries.ListReplacementsByRequester(r.Context(), h.currentUser(r).ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "loading requests")
		return
	}
	respondJSON(w, http.StatusOK, rs)
}

// ListIncomingReplacements returns pending requests to replace the current user.
func (h *Handler) ListIncomingReplacements(w http.ResponseWriter, r *http.Request) {
	rs, err := h.queries.ListReplacementsForIncumbent(r.Context(), h.currentUser(r).ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "loading requests")
		return
	}
	respondJSON(w, http.StatusOK, rs)
}

// DecideReplacement approves or rejects a request. The incumbent being replaced
// may decide their own slot; managers/administrators may decide any.
func (h *Handler) DecideReplacement(w http.ResponseWriter, r *http.Request, approve bool) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	u := h.currentUser(r)
	rep, err := h.queries.GetContactReplacement(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "request not found")
		return
	}
	if rep.Status != "pending" {
		respondError(w, http.StatusConflict, "request already decided")
		return
	}
	isIncumbent := rep.IncumbentUser != "" && rep.IncumbentUser == u.ID
	if !isIncumbent && !h.isManagerOrAdmin(r) {
		respondError(w, http.StatusForbidden, "only the current contact or a manager can decide this")
		return
	}
	status := "rejected"
	if approve {
		if err := h.queries.ReplaceContactSlot(ctx, rep.EntityKind, rep.EntityName, rep.ContactType, rep.Rank,
			rep.RequesterName, h.primaryCILogonID(ctx, rep.RequesterUser), rep.RequesterUser, u.ID); err != nil {
			respondError(w, http.StatusInternalServerError, "applying replacement: "+err.Error())
			return
		}
		status = "approved"
	}
	if err := h.queries.DecideContactReplacement(ctx, id, status, u.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "recording decision")
		return
	}
	h.audit(ctx, u.ID, "replacement."+status, rep.EntityKind, rep.EntityName, id, nil)
	respondJSON(w, http.StatusOK, map[string]string{"status": status})
}

// ApproveReplacement / RejectReplacement / WithdrawReplacement are the routed
// handlers around DecideReplacement.
func (h *Handler) ApproveReplacement(w http.ResponseWriter, r *http.Request) {
	h.DecideReplacement(w, r, true)
}
func (h *Handler) RejectReplacement(w http.ResponseWriter, r *http.Request) {
	h.DecideReplacement(w, r, false)
}

// WithdrawReplacement lets the requester cancel their own pending request.
func (h *Handler) WithdrawReplacement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	u := h.currentUser(r)
	rep, err := h.queries.GetContactReplacement(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "request not found")
		return
	}
	if rep.RequesterUser != u.ID {
		respondError(w, http.StatusForbidden, "only the requester can withdraw this")
		return
	}
	if err := h.queries.DecideContactReplacement(ctx, id, "withdrawn", u.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "withdrawing")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "withdrawn"})
}
