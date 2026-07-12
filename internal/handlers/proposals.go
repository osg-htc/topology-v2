package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
	"github.com/bbockelm/topology-v2/internal/proposalschema"
	"github.com/bbockelm/topology-v2/internal/topology"
)

var (
	errUnsupportedKind  = errors.New("this entity kind is not yet supported for automatic apply")
	errBadProposalState = errors.New("proposed_state must include name and resource_group")
)

// isManagerOrAdmin reports whether the current session may review/override.
func (h *Handler) isManagerOrAdmin(r *http.Request) bool {
	role := models.EffectiveRole(rolesFromContext(r.Context()))
	return role == models.RoleManager || role == models.RoleAdministrator
}

type createProposalRequest struct {
	EntityKind    string          `json:"entity_kind"`
	Operation     string          `json:"operation"`
	TargetName    string          `json:"target_name"`
	ProposedState json.RawMessage `json:"proposed_state"`
	Note          string          `json:"note"`
	Submit        bool            `json:"submit"` // true = go straight to pending
}

// CreateProposal creates a draft (or pending) change proposal.
func (h *Handler) CreateProposal(w http.ResponseWriter, r *http.Request) {
	var req createProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	switch req.Operation {
	case models.OpCreate, models.OpUpdate, models.OpDelete:
	default:
		respondError(w, http.StatusBadRequest, "invalid operation")
		return
	}
	if req.EntityKind == "" {
		respondError(w, http.StatusBadRequest, "entity_kind required")
		return
	}
	u := h.currentUser(r)
	ctx := r.Context()

	// Validate the payload against the current JSON Schema for this kind
	// (delete carries no payload, so skip it there).
	schemaVer := 0
	if req.Operation != models.OpDelete && proposalschema.Known(req.EntityKind) {
		schemaVer = proposalschema.CurrentVersion(req.EntityKind)
		if err := proposalschema.Validate(req.EntityKind, schemaVer, orEmptyJSON(req.ProposedState)); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	status := models.ProposalDraft
	if req.Submit {
		status = models.ProposalPending
	}
	// Snapshot the live entity as base_version for update/delete (conflict detection).
	var base json.RawMessage
	if req.Operation != models.OpCreate && req.TargetName != "" && req.EntityKind == models.KindResource {
		base = h.snapshotResource(ctx, req.TargetName)
	}

	id, err := h.queries.CreateProposal(ctx, db.CreateProposalParams{
		EntityKind: req.EntityKind, TargetName: req.TargetName, Operation: req.Operation,
		ProposedState: req.ProposedState, SchemaVersion: schemaVer, BaseVersion: base,
		Status: status, CreatedBy: u.ID, Note: req.Note,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "creating proposal")
		return
	}
	h.audit(ctx, u.ID, "proposal.create", req.EntityKind, req.TargetName, id, nil)
	respondJSON(w, http.StatusCreated, map[string]string{"id": id, "status": status})
}

type reviseRequest struct {
	ProposedState json.RawMessage `json:"proposed_state"`
	Note          string          `json:"note"`
}

// ReviseProposal appends a revision. The creator may revise their own draft or
// pending proposal; managers/admins may revise (override) any.
func (h *Handler) ReviseProposal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	p, err := h.queries.GetProposal(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "proposal not found")
		return
	}
	u := h.currentUser(r)
	if p.CreatedBy != u.ID && !h.isManagerOrAdmin(r) {
		respondError(w, http.StatusForbidden, "not allowed to edit this proposal")
		return
	}
	if p.Status == models.ProposalApplied || p.Status == models.ProposalRejected {
		respondError(w, http.StatusConflict, "proposal is closed")
		return
	}
	var req reviseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	// Edits always target the current schema version.
	if p.Operation != models.OpDelete && proposalschema.Known(p.EntityKind) {
		ver := proposalschema.CurrentVersion(p.EntityKind)
		if err := proposalschema.Validate(p.EntityKind, ver, orEmptyJSON(req.ProposedState)); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if err := h.queries.AddRevision(ctx, id, req.ProposedState, u.ID, req.Note); err != nil {
		respondError(w, http.StatusInternalServerError, "saving revision")
		return
	}
	h.audit(ctx, u.ID, "proposal.revise", p.EntityKind, p.TargetName, id, nil)
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// SubmitProposal moves a draft to pending (creator only).
func (h *Handler) SubmitProposal(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, models.ProposalPending, false, "proposal.submit")
}

// WithdrawProposal withdraws a proposal (creator only).
func (h *Handler) WithdrawProposal(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, models.ProposalWithdrawn, false, "proposal.withdraw")
}

// RejectProposal rejects a pending proposal (manager/admin only).
func (h *Handler) RejectProposal(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, models.ProposalRejected, true, "proposal.reject")
}

// transition handles simple status changes with authorization.
func (h *Handler) transition(w http.ResponseWriter, r *http.Request, target string, requireReviewer bool, action string) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	p, err := h.queries.GetProposal(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "proposal not found")
		return
	}
	u := h.currentUser(r)
	if requireReviewer {
		if !h.isManagerOrAdmin(r) {
			respondError(w, http.StatusForbidden, "requires manager or administrator")
			return
		}
	} else if p.CreatedBy != u.ID {
		respondError(w, http.StatusForbidden, "only the creator may do this")
		return
	}
	var note string
	if requireReviewer {
		var body struct {
			Note string `json:"note"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		note = body.Note
	}
	reviewer := ""
	if requireReviewer {
		reviewer = u.ID
	}
	if err := h.queries.UpdateProposalStatus(ctx, id, target, reviewer, note); err != nil {
		respondError(w, http.StatusInternalServerError, "updating status")
		return
	}
	h.audit(ctx, u.ID, action, p.EntityKind, p.TargetName, id, nil)
	respondJSON(w, http.StatusOK, map[string]string{"status": target})
}

// ApproveProposal applies an approved proposal to the live tables (manager/admin).
func (h *Handler) ApproveProposal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	if !h.isManagerOrAdmin(r) {
		respondError(w, http.StatusForbidden, "requires manager or administrator")
		return
	}
	p, err := h.queries.GetProposal(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "proposal not found")
		return
	}
	if p.Status != models.ProposalPending {
		respondError(w, http.StatusConflict, "only pending proposals can be approved")
		return
	}
	u := h.currentUser(r)
	if err := h.applyProposal(ctx, p, u.ID); err != nil {
		respondError(w, http.StatusBadRequest, "applying proposal: "+err.Error())
		return
	}
	if err := h.queries.UpdateProposalStatus(ctx, id, models.ProposalApplied, u.ID, ""); err != nil {
		respondError(w, http.StatusInternalServerError, "updating status")
		return
	}
	h.audit(ctx, u.ID, "proposal.approve", p.EntityKind, p.TargetName, id, p.ProposedState)
	respondJSON(w, http.StatusOK, map[string]string{"status": models.ProposalApplied})
}

// GetProposal returns a proposal with its full revision history.
func (h *Handler) GetProposal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	p, err := h.queries.GetProposal(ctx, id)
	if err != nil {
		respondError(w, http.StatusNotFound, "proposal not found")
		return
	}
	if revs, err := h.queries.ListRevisions(ctx, id); err == nil {
		p.Revisions = revs
	}
	respondJSON(w, http.StatusOK, p)
}

// ListMyProposals lists the current user's proposals.
func (h *Handler) ListMyProposals(w http.ResponseWriter, r *http.Request) {
	ps, err := h.queries.ListProposalsByCreator(r.Context(), h.currentUser(r).ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "loading proposals")
		return
	}
	respondJSON(w, http.StatusOK, ps)
}

// ListPendingProposals lists all pending proposals (reviewer queue).
func (h *Handler) ListPendingProposals(w http.ResponseWriter, r *http.Request) {
	if !h.isManagerOrAdmin(r) {
		respondError(w, http.StatusForbidden, "requires manager or administrator")
		return
	}
	ps, err := h.queries.ListPendingProposals(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "loading proposals")
		return
	}
	respondJSON(w, http.StatusOK, ps)
}

// ListAuditHandler returns recent audit-log entries (manager/admin only).
func (h *Handler) ListAuditHandler(w http.ResponseWriter, r *http.Request) {
	entries, err := h.queries.ListAudit(r.Context(), 100)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "loading audit log")
		return
	}
	respondJSON(w, http.StatusOK, entries)
}

// ---- apply ----

// resourceProposal is the proposed_state shape for a resource change.
type resourceProposal struct {
	ResourceGroup string            `json:"resource_group"`
	Name          string            `json:"name"`
	Resource      topology.Resource `json:"resource"`
}

// applyProposal dispatches by entity kind. Resources (the primary entity) are
// fully supported; other kinds are added as the workflow expands.
func (h *Handler) applyProposal(ctx context.Context, p *models.Proposal, actorID string) error {
	if p.EntityKind != models.KindResource {
		return errUnsupportedKind
	}
	// Bring the payload forward to the current schema (no-op if already current)
	// and re-validate before touching live tables. This is where a proposal that
	// predates a schema change gets upgraded.
	if p.Operation != models.OpDelete && proposalschema.Known(p.EntityKind) {
		ver := p.SchemaVersion
		if ver == 0 {
			ver = 1
		}
		upgraded, _, err := proposalschema.Upgrade(p.EntityKind, ver, orEmptyJSON(p.ProposedState))
		if err != nil {
			return err
		}
		p.ProposedState = upgraded
	}
	return h.applyResourceProposal(ctx, p, actorID)
}

// orEmptyJSON returns b, or an empty JSON object if b is empty.
func orEmptyJSON(b json.RawMessage) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

func (h *Handler) applyResourceProposal(ctx context.Context, p *models.Proposal, actorID string) error {
	// Delete: soft-delete the target resource.
	if p.Operation == models.OpDelete {
		id, err := h.queries.ResourceIDByName(ctx, p.TargetName)
		if err != nil {
			return err
		}
		return h.queries.SoftDeleteResource(ctx, id, actorID)
	}

	var rp resourceProposal
	if err := json.Unmarshal(p.ProposedState, &rp); err != nil {
		return err
	}
	if rp.Name == "" || rp.ResourceGroup == "" {
		return errBadProposalState
	}
	rgID, err := h.queries.ResourceGroupIDByName(ctx, rp.ResourceGroup)
	if err != nil {
		return err
	}
	// Update: soft-delete the existing version first (versioned via delete+insert).
	if p.Operation == models.OpUpdate {
		if id, err := h.queries.ResourceIDByName(ctx, rp.Name); err == nil {
			if err := h.queries.SoftDeleteResource(ctx, id, actorID); err != nil {
				return err
			}
		}
	}
	_, err = topology.UpsertResource(ctx, h.queries, rgID, rp.Name, &rp.Resource)
	return err
}

// snapshotResource captures the current resource state as base_version.
func (h *Handler) snapshotResource(ctx context.Context, name string) json.RawMessage {
	id, err := h.queries.ResourceIDByName(ctx, name)
	if err != nil {
		return nil
	}
	b, _ := json.Marshal(map[string]string{"resource_id": id, "name": name})
	return b
}

// audit is a convenience wrapper that never blocks the request on failure.
func (h *Handler) audit(ctx context.Context, actor, action, kind, entityID, proposalID string, detail json.RawMessage) {
	_ = h.queries.AppendAudit(ctx, actor, action, kind, entityID, proposalID, detail)
}
