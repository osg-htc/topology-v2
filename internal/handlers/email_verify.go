package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/bbockelm/topology-v2/internal/db"
)

const emailVerifyExpiry = 48 * time.Hour

type requestEmailVerifyRequest struct {
	Email string `json:"email"`
}

// RequestEmailVerification issues a single-use link to verify that the current
// user controls an email address. The plaintext is never persisted (only its
// hash, a masked hint, and the envelope-encrypted ciphertext). Delivery is
// stubbed: the link is logged, and in development it is returned so the flow can
// be exercised without an SMTP server.
func (h *Handler) RequestEmailVerification(w http.ResponseWriter, r *http.Request) {
	var req requestEmailVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	email := strings.TrimSpace(req.Email)
	if !looksLikeEmail(email) {
		respondError(w, http.StatusBadRequest, "a valid email address is required")
		return
	}
	ctx := r.Context()
	u := h.currentUser(r)

	token := randomToken(32)
	params := db.UpsertEmailVerificationParams{
		UserID:    u.ID,
		EmailSHA1: emailSHA1(email),
		EmailHint: maskEmail(email),
		TokenHash: hashToken(token),
		ExpiresAt: time.Now().Add(emailVerifyExpiry),
	}
	if ef, err := h.encryptor.EncryptPII(email); err == nil {
		params.EmailCiphertext = ef.Ciphertext
		params.EmailDEKWrapped = ef.WrappedDEK
	}
	if err := h.queries.UpsertEmailVerification(ctx, params); err != nil {
		respondError(w, http.StatusInternalServerError, "storing verification")
		return
	}
	verifyURL := h.cfg.BaseURL + "/verify-email?token=" + token
	// Delivery stub: in production this is where the mail would be sent.
	log.Info().Str("user", u.ID).Str("email", params.EmailHint).Str("url", verifyURL).
		Msg("email verification link issued")
	h.audit(ctx, u.ID, "email.verify.request", "", "", "", nil)

	resp := map[string]string{"status": "sent", "email": params.EmailHint}
	if !h.cfg.IsProduction() {
		resp["verify_url"] = verifyURL // convenience for local/dev use only
	}
	respondJSON(w, http.StatusCreated, resp)
}

type confirmEmailVerifyRequest struct {
	Token string `json:"token"`
}

// ConfirmEmailVerification marks the email behind a token verified.
func (h *Handler) ConfirmEmailVerification(w http.ResponseWriter, r *http.Request) {
	var req confirmEmailVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Token == "" {
		respondError(w, http.StatusBadRequest, "token required")
		return
	}
	ctx := r.Context()
	hint, ok := h.queries.ConfirmEmailVerification(ctx, hashToken(req.Token))
	if !ok {
		respondError(w, http.StatusGone, "link is invalid, expired, or already used")
		return
	}
	h.audit(ctx, h.currentUser(r).ID, "email.verify.confirm", "", "", "", nil)
	respondJSON(w, http.StatusOK, map[string]string{"status": "verified", "email": hint})
}

// ListEmailVerifications returns the current user's emails and their status.
func (h *Handler) ListEmailVerifications(w http.ResponseWriter, r *http.Request) {
	list, err := h.queries.ListEmailVerifications(r.Context(), h.currentUser(r).ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "loading emails")
		return
	}
	respondJSON(w, http.StatusOK, list)
}

// looksLikeEmail is a lightweight sanity check (real validation is control of
// the address, proven by the link).
func looksLikeEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return false
	}
	return strings.IndexByte(s[at+1:], '.') > 0 && !strings.ContainsAny(s, " \t\r\n")
}

// maskEmail turns "jane.doe@example.org" into "j***@example.org".
func maskEmail(s string) string {
	at := strings.IndexByte(s, '@')
	if at <= 0 {
		return "***"
	}
	local, domain := s[:at], s[at:]
	if len(local) <= 1 {
		return local + "***" + domain
	}
	return local[:1] + "***" + domain
}
