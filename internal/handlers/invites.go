package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
)

type createInviteRequest struct {
	Kind         string            `json:"kind"`
	TargetUserID string            `json:"target_user_id"`
	Claim        *models.RoleClaim `json:"claim"`
}

// CreateInvite issues a single-use invite link. account_link invites (linking a
// new federated identity to an existing account) require manager/admin;
// role_claim invites (offering a contact responsibility) may be created by any
// authenticated user — an owner inviting someone to take a role.
func (h *Handler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	var req createInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	ctx := r.Context()
	u := h.currentUser(r)

	var claimJSON []byte
	switch req.Kind {
	case models.InviteAccountLink:
		if !h.isManagerOrAdmin(r) {
			respondError(w, http.StatusForbidden, "requires manager or administrator")
			return
		}
		if req.TargetUserID == "" {
			respondError(w, http.StatusBadRequest, "target_user_id required")
			return
		}
	case models.InviteRoleClaim:
		if req.Claim == nil || req.Claim.EntityID == "" || req.Claim.ContactType == "" {
			respondError(w, http.StatusBadRequest, "claim with entity_id and contact_type required")
			return
		}
		if req.Claim.Rank == "" {
			req.Claim.Rank = "Primary"
		}
		claimJSON, _ = json.Marshal(req.Claim)
	default:
		respondError(w, http.StatusBadRequest, "invalid invite kind")
		return
	}

	token := randomToken(32)
	_, err := h.queries.CreateInvite(ctx, db.CreateInviteParams{
		Kind: req.Kind, TokenHash: hashToken(token), CreatedBy: u.ID,
		TargetUserID: req.TargetUserID, ClaimJSON: claimJSON,
		ExpiresAt: time.Now().Add(inviteExpiry),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "creating invite")
		return
	}
	h.audit(ctx, u.ID, "invite.create", req.Kind, req.TargetUserID, "", claimJSON)
	respondJSON(w, http.StatusCreated, map[string]string{
		"invite_url": h.cfg.BaseURL + "/invites/" + token,
		"token":      token,
	})
}
