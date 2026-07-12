package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/bbockelm/topology-v2/internal/crypto"
	"github.com/bbockelm/topology-v2/internal/models"
)

// ---- OIDC settings (administrator) ----

type oidcConfigDTO struct {
	Issuer    string `json:"issuer"`
	ClientID  string `json:"client_id"`
	HasSecret bool   `json:"has_secret"`
}

// GetOIDCConfig returns the current OIDC settings (secret presence only).
func (h *Handler) GetOIDCConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	secret, _ := h.queries.GetConfig(ctx, "oidc_client_secret")
	respondJSON(w, http.StatusOK, oidcConfigDTO{
		Issuer:    h.configOrDefault(ctx, "oidc_issuer", h.cfg.OIDCIssuer),
		ClientID:  h.configOrDefault(ctx, "oidc_client_id", h.cfg.OIDCClientID),
		HasSecret: secret != "" || h.cfg.OIDCClientSecret != "",
	})
}

type setOIDCConfigRequest struct {
	Issuer       string  `json:"issuer"`
	ClientID     string  `json:"client_id"`
	ClientSecret *string `json:"client_secret"` // nil = leave unchanged
}

// SetOIDCConfig updates OIDC settings in app_config; the secret is encrypted.
func (h *Handler) SetOIDCConfig(w http.ResponseWriter, r *http.Request) {
	var req setOIDCConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	ctx := r.Context()
	if err := h.queries.SetConfig(ctx, "oidc_issuer", req.Issuer); err != nil {
		respondError(w, http.StatusInternalServerError, "saving issuer")
		return
	}
	if err := h.queries.SetConfig(ctx, "oidc_client_id", req.ClientID); err != nil {
		respondError(w, http.StatusInternalServerError, "saving client id")
		return
	}
	if req.ClientSecret != nil && *req.ClientSecret != "" {
		enc, err := h.encryptor.EncryptConfigValue(*req.ClientSecret)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "encrypting secret")
			return
		}
		if err := h.queries.SetConfig(ctx, "oidc_client_secret", enc); err != nil {
			respondError(w, http.StatusInternalServerError, "saving secret")
			return
		}
	}
	h.audit(ctx, h.currentUser(r).ID, "settings.oidc_update", "config", "oidc", "", nil)
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---- user management (administrator) ----

type adminUserDTO struct {
	models.User
	Identities []adminIdentityDTO `json:"identities"`
}
type adminIdentityDTO struct {
	ID        string `json:"id"`
	Issuer    string `json:"issuer"`
	Subject   string `json:"subject"`
	Email     string `json:"email,omitempty"`
	CILogonID string `json:"cilogon_id,omitempty"`
}

// ListUsersHandler returns all users with roles and identities (admin view; the
// admin is authorized to see contact emails, so they are decrypted).
func (h *Handler) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	users, err := h.queries.ListAllUsers(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]adminUserDTO, 0, len(users))
	for _, u := range users {
		roles, _ := h.queries.GetUserRoles(ctx, u.ID)
		if roles == nil {
			roles = []string{}
		}
		u.Roles = roles
		dto := adminUserDTO{User: *u, Identities: []adminIdentityDTO{}}
		rows, _ := h.queries.ListUserIdentities(ctx, u.ID)
		for _, row := range rows {
			id := row.Identity()
			ad := adminIdentityDTO{ID: id.ID, Issuer: id.Issuer, Subject: id.Subject, CILogonID: id.CILogonID}
			if ct, wrapped := row.Encrypted(); len(ct) > 0 {
				if email, err := h.encryptor.DecryptPII(&crypto.EncryptedField{Ciphertext: ct, WrappedDEK: wrapped}); err == nil {
					ad.Email = email
				}
			}
			dto.Identities = append(dto.Identities, ad)
		}
		out = append(out, dto)
	}
	respondJSON(w, http.StatusOK, out)
}

type roleChangeRequest struct {
	Role   string `json:"role"`
	Action string `json:"action"` // add | remove
}

// SetUserRoleHandler grants or revokes a role for a user.
func (h *Handler) SetUserRoleHandler(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req roleChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	switch req.Role {
	case models.RoleAdministrator, models.RoleManager, models.RoleUser:
	default:
		respondError(w, http.StatusBadRequest, "invalid role")
		return
	}
	ctx := r.Context()
	var err error
	switch req.Action {
	case "add":
		err = h.queries.AddUserRole(ctx, userID, req.Role)
	case "remove":
		err = h.queries.RemoveUserRole(ctx, userID, req.Role)
	default:
		respondError(w, http.StatusBadRequest, "action must be add or remove")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "updating role")
		return
	}
	h.audit(ctx, h.currentUser(r).ID, "user.role_"+req.Action, "user", userID, "", nil)
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
