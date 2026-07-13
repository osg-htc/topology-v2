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
	// contact_onboard: the new person's display name (and optional email).
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
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
	// contact_onboard returns extra fields describing the freshly-provisioned user.
	var onboardedUserID, onboardedName string
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
	case models.InviteRoleClaim, models.InviteReplacementRequest:
		if req.Claim == nil || req.Claim.EntityID == "" || req.Claim.ContactType == "" {
			respondError(w, http.StatusBadRequest, "claim with entity_id and contact_type required")
			return
		}
		if req.Claim.Rank == "" {
			req.Claim.Rank = "Primary"
		}
		claimJSON, _ = json.Marshal(req.Claim)
	case models.InviteContactOnboard:
		// Any authenticated user may onboard a brand-new contact: provision a
		// user now (no linked identity) and target the invite at it. When the
		// invitee signs in through the link, their identity attaches to it.
		if req.DisplayName == "" {
			respondError(w, http.StatusBadRequest, "display_name required")
			return
		}
		uid, err := h.queries.CreateUser(ctx, db.CreateUserParams{
			DisplayName: req.DisplayName, Status: "active", IsProvisioned: true,
		})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "provisioning contact")
			return
		}
		req.TargetUserID = uid
		onboardedUserID, onboardedName = uid, req.DisplayName
	default:
		respondError(w, http.StatusBadRequest, "invalid invite kind")
		return
	}

	token := randomToken(32)
	inviteID, err := h.queries.CreateInvite(ctx, db.CreateInviteParams{
		Kind: req.Kind, TokenHash: hashToken(token), CreatedBy: u.ID,
		TargetUserID: req.TargetUserID, ClaimJSON: claimJSON,
		ExpiresAt: time.Now().Add(inviteExpiry),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "creating invite")
		return
	}
	h.audit(ctx, u.ID, "invite.create", req.Kind, req.TargetUserID, "", claimJSON)
	// account_link / contact_onboard are consumed by signing in through the link
	// (identity attaches during login); the others use the accept page.
	acceptPath := "/invites/accept?token="
	if req.Kind == models.InviteAccountLink || req.Kind == models.InviteContactOnboard {
		acceptPath = "/login?invite="
	}
	resp := map[string]string{
		"invite_url": h.cfg.BaseURL + acceptPath + token,
		"token":      token,
		"invite_id":  inviteID,
	}
	if onboardedUserID != "" {
		resp["user_id"] = onboardedUserID
		resp["name"] = onboardedName
	}
	respondJSON(w, http.StatusCreated, resp)
}
