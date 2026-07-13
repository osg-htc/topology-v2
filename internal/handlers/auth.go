package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/topology-v2/internal/crypto"
	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/models"
)

const (
	sessionDuration   = 7 * 24 * time.Hour
	inviteExpiry      = 7 * 24 * time.Hour
	sessionTokenBytes = 32
	oidcStateCookie   = "topology_oidc_state"
	oidcNameCookie    = "topology_oidc_name"
)

// AuthMode reports how clients should authenticate (dev vs oidc).
func (h *Handler) AuthMode(w http.ResponseWriter, r *http.Request) {
	mode := "oidc"
	if !h.cfg.IsProduction() {
		mode = "dev"
	}
	respondJSON(w, http.StatusOK, map[string]string{"mode": mode})
}

// ---- OIDC ----

type oidcEndpoints struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	AuthURL      string `json:"authorization_endpoint"`
	TokenURL     string `json:"token_endpoint"`
	UserInfoURL  string `json:"userinfo_endpoint"`
}

// resolveOIDC loads OIDC settings from app_config (secret decrypted) falling
// back to env config, then fetches the provider's discovery document.
func (h *Handler) resolveOIDC(ctx context.Context) (*oidcEndpoints, error) {
	issuer := h.configOrDefault(ctx, "oidc_issuer", h.cfg.OIDCIssuer)
	clientID := h.configOrDefault(ctx, "oidc_client_id", h.cfg.OIDCClientID)
	clientSecret := h.cfg.OIDCClientSecret
	if enc, _ := h.queries.GetConfig(ctx, "oidc_client_secret"); enc != "" {
		if dec, err := h.encryptor.DecryptConfigValue(enc); err == nil && dec != "" {
			clientSecret = dec
		}
	}
	if clientID == "" {
		return nil, errors.New("OIDC client id not configured")
	}

	discoveryURL := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching OIDC discovery: %w", err)
	}
	defer resp.Body.Close()
	ep := &oidcEndpoints{Issuer: issuer, ClientID: clientID, ClientSecret: clientSecret}
	if err := json.NewDecoder(resp.Body).Decode(ep); err != nil {
		return nil, fmt.Errorf("decoding OIDC discovery: %w", err)
	}
	return ep, nil
}

func (h *Handler) configOrDefault(ctx context.Context, key, def string) string {
	if v, _ := h.queries.GetConfig(ctx, key); v != "" {
		return v
	}
	return def
}

func buildScopes(issuer string) string {
	scopes := "openid email profile"
	if strings.Contains(issuer, "cilogon.org") {
		scopes += " org.cilogon.userinfo"
	}
	return scopes
}

// OIDCLogin redirects the browser to the provider's authorization endpoint.
// A random state (plus optional invite token and return_to) is HMAC-signed and
// stored in a short-lived cookie; only the random state is sent to the provider.
func (h *Handler) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ep, err := h.resolveOIDC(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	state := randomToken(16)
	invite := r.URL.Query().Get("invite")
	returnTo := r.URL.Query().Get("return_to")
	payload := strings.Join([]string{state, invite, returnTo}, "|")
	sig := crypto.SignHMAC(h.sessionSecret, payload)
	cookieVal := sig + ":" + base64.RawURLEncoding.EncodeToString([]byte(payload))
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    cookieVal,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.IsProduction(),
		MaxAge:   600,
	})

	q := url.Values{}
	q.Set("client_id", ep.ClientID)
	q.Set("response_type", "code")
	q.Set("scope", buildScopes(ep.Issuer))
	q.Set("redirect_uri", h.cfg.BaseURL+"/api/v1/auth/oidc/callback")
	q.Set("state", state)
	http.Redirect(w, r, ep.AuthURL+"?"+q.Encode(), http.StatusFound)
}

// OIDCCallback completes the auth code flow, resolving or linking a federated
// identity and issuing a session.
func (h *Handler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.URL.Query().Get("code")
	stateParam := r.URL.Query().Get("state")
	if code == "" {
		h.redirectLogin(w, r, "missing_code")
		return
	}

	// Verify signed state cookie and recover invite/return_to.
	invite, returnTo, ok := h.verifyState(r, stateParam)
	if !ok {
		h.redirectLogin(w, r, "bad_state")
		return
	}
	clearCookie(w, oidcStateCookie, h.cfg.IsProduction())

	ep, err := h.resolveOIDC(ctx)
	if err != nil {
		h.redirectLogin(w, r, "oidc_config")
		return
	}
	tok, err := h.exchangeCode(ctx, ep, code)
	if err != nil {
		log.Error().Err(err).Msg("oidc code exchange failed")
		h.redirectLogin(w, r, "exchange_failed")
		return
	}

	claims := parseIDToken(tok.IDToken)
	issuer := stringClaim(claims, "iss")
	if issuer == "" {
		issuer = ep.Issuer
	}
	subject := stringClaim(claims, "sub")
	email := stringClaim(claims, "email")
	name := stringClaim(claims, "name")

	// Pull richer claims (CILogon) from userinfo.
	cilogonID, eppn, oidcClaim, idpName := "", "", "", ""
	if info := h.fetchUserInfo(ctx, ep, tok.AccessToken); info != nil {
		if email == "" {
			email = stringClaim(info, "email")
		}
		if name == "" {
			name = stringClaim(info, "name")
		}
		cilogonID = firstNonEmpty(stringClaim(info, "id"), stringClaim(info, "sub"))
		eppn = stringClaim(info, "eppn")
		oidcClaim = stringClaim(info, "oidc")
		idpName = stringClaim(info, "idp_name")
	}
	if subject == "" {
		h.redirectLogin(w, r, "no_subject")
		return
	}

	// Set a short-lived, readable cookie so the welcome page can prefill name.
	if name != "" {
		http.SetCookie(w, &http.Cookie{
			Name: oidcNameCookie, Value: url.QueryEscape(name), Path: "/",
			MaxAge: 300, SameSite: http.SameSiteLaxMode, Secure: h.cfg.IsProduction(),
		})
	}

	// 1) Existing identity → log in.
	if existing, err := h.queries.FindIdentity(ctx, issuer, subject); err == nil {
		h.loginUser(ctx, w, existing.UserID)
		h.finishRedirect(w, r, returnTo, invite)
		return
	}

	// 2) No identity yet. Decide how to onboard.
	userID, ferr := h.onboardIdentity(ctx, w, r, onboardParams{
		Issuer: issuer, Subject: subject, Email: email, Name: name,
		CILogonID: cilogonID, EPPN: eppn, OIDC: oidcClaim, IdPName: idpName,
		InviteToken: invite,
	})
	if ferr != nil {
		h.redirectLogin(w, r, ferr.Error())
		return
	}
	if userID == "" {
		// No account and no invite: nothing to log into.
		h.redirectLogin(w, r, "no_account")
		return
	}
	h.loginUser(ctx, w, userID)
	h.finishRedirect(w, r, returnTo, invite)
}

type onboardParams struct {
	Issuer, Subject, Email, Name          string
	CILogonID, EPPN, OIDC, IdPName        string
	InviteToken                           string
}

// onboardIdentity links a brand-new federated identity to an account. Order of
// preference: an account_link invite target; a provisioned contact matched by
// CILogon id; a fresh account created for a role_claim invite. Returns the
// resolved user id (empty if there's nothing to onboard into).
func (h *Handler) onboardIdentity(ctx context.Context, w http.ResponseWriter, r *http.Request, p onboardParams) (string, error) {
	var inv *db.InviteRow
	if p.InviteToken != "" {
		got, err := h.queries.GetInviteByTokenHash(ctx, hashToken(p.InviteToken))
		if err == nil && got.UsedAt == nil && got.ExpiresAt.After(time.Now()) {
			inv = got
		}
	}

	// account_link invite → attach identity to the pre-provisioned target.
	if inv != nil && inv.Kind == models.InviteAccountLink && inv.TargetUserID != "" {
		if err := h.linkIdentity(ctx, inv.TargetUserID, p); err != nil {
			return "", err
		}
		_ = h.queries.MarkInviteUsed(ctx, inv.ID, inv.TargetUserID)
		return inv.TargetUserID, nil
	}

	// Match a provisioned contact by CILogon id (same federated identity).
	if p.CILogonID != "" {
		if u, err := h.queries.FindUserByLegacyContactID(ctx, p.CILogonID); err == nil {
			if err := h.linkIdentity(ctx, u.ID, p); err != nil {
				return "", err
			}
			return u.ID, nil
		}
	}

	// role_claim invite → create a fresh account, then link. The claim itself is
	// accepted in a separate authenticated step (AcceptInvite).
	if inv != nil && inv.Kind == models.InviteRoleClaim {
		newID, err := h.queries.CreateUser(ctx, db.CreateUserParams{DisplayName: p.Name, Status: "active"})
		if err != nil {
			return "", errors.New("create_user_failed")
		}
		_ = h.queries.AddUserRole(ctx, newID, models.RoleUser)
		if err := h.linkIdentity(ctx, newID, p); err != nil {
			return "", err
		}
		return newID, nil
	}

	return "", nil
}

// linkIdentity encrypts the email and inserts a user_identities row, translating
// a unique-violation into a friendly "already linked" error.
func (h *Handler) linkIdentity(ctx context.Context, userID string, p onboardParams) error {
	params := db.CreateIdentityParams{
		UserID: userID, Issuer: p.Issuer, Subject: p.Subject,
		EPPN: p.EPPN, OIDC: p.OIDC, CILogonID: p.CILogonID,
		IdPName: p.IdPName, DisplayName: p.Name,
	}
	if p.Email != "" {
		params.EmailSHA1 = emailSHA1(p.Email)
		if ef, err := h.encryptor.EncryptPII(p.Email); err == nil {
			params.EmailCiphertext = ef.Ciphertext
			params.EmailDEKWrapped = ef.WrappedDEK
		}
	}
	if _, err := h.queries.CreateIdentity(ctx, params); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errors.New("identity_already_linked")
		}
		return errors.New("link_failed")
	}
	return nil
}

// loginUser stamps last-login, ensures a username, snapshots the effective
// role, and sets the session cookie.
func (h *Handler) loginUser(ctx context.Context, w http.ResponseWriter, userID string) {
	_ = h.queries.UpdateUserLastLogin(ctx, userID)
	// Derive a unique username at first login (idempotent thereafter). Base it on
	// the user's display name (a proxy for the preferred_username claim).
	if u, err := h.queries.GetUser(ctx, userID); err == nil {
		base := u.DisplayName
		if base == "" {
			base = "user"
		}
		_, _ = h.queries.EnsureUsername(ctx, userID, base)
	}
	roles, _ := h.queries.GetUserRoles(ctx, userID)
	if len(roles) == 0 {
		_ = h.queries.AddUserRole(ctx, userID, models.RoleUser)
		roles = []string{models.RoleUser}
	}
	h.createSession(ctx, w, userID, models.EffectiveRole(roles))
}

// createSession issues a fresh opaque session and sets the cookie.
func (h *Handler) createSession(ctx context.Context, w http.ResponseWriter, userID, role string) {
	raw := randomToken(sessionTokenBytes)
	_, err := h.queries.CreateSession(ctx, userID, role, hashToken(raw), time.Now().Add(sessionDuration))
	if err != nil {
		log.Error().Err(err).Msg("creating session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.IsProduction(),
		MaxAge:   int(sessionDuration.Seconds()),
	})
}

// ---- token exchange helpers ----

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
}

func (h *Handler) exchangeCode(ctx context.Context, ep *oidcEndpoints, code string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", h.cfg.BaseURL+"/api/v1/auth/oidc/callback")
	form.Set("client_id", ep.ClientID)
	form.Set("client_secret", ep.ClientSecret)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ep.TokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token endpoint status %d: %s", resp.StatusCode, string(body))
	}
	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, err
	}
	return &tr, nil
}

func (h *Handler) fetchUserInfo(ctx context.Context, ep *oidcEndpoints, accessToken string) map[string]any {
	if ep.UserInfoURL == "" || accessToken == "" {
		return nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ep.UserInfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var info map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil
	}
	return info
}

// parseIDToken base64-decodes a JWT payload. The signature is NOT verified: the
// token is received directly from the trusted token endpoint over TLS in the
// same request. (If ID tokens can ever arrive via the front channel, verify.)
func parseIDToken(idToken string) map[string]any {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return claims
}

// ---- dev login ----

type devLoginRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

// DevLogin issues a session without OIDC. Only available in development.
func (h *Handler) DevLogin(w http.ResponseWriter, r *http.Request) {
	if h.cfg.IsProduction() {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	var req devLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Email == "" {
		req.Email = "dev@example.org"
	}
	if req.DisplayName == "" {
		req.DisplayName = "Dev User"
	}
	role := req.Role
	if role == "" {
		role = models.RoleAdministrator
	}
	ctx := r.Context()
	issuer, subject := "dev", req.Email

	var userID string
	if existing, err := h.queries.FindIdentity(ctx, issuer, subject); err == nil {
		userID = existing.UserID
	} else {
		userID, err = h.queries.CreateUser(ctx, db.CreateUserParams{DisplayName: req.DisplayName, Status: "active"})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "creating user")
			return
		}
		_ = h.linkIdentity(ctx, userID, onboardParams{Issuer: issuer, Subject: subject, Email: req.Email, Name: req.DisplayName})
	}
	_ = h.queries.AddUserRole(ctx, userID, role)
	h.loginUser(ctx, w, userID)
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok", "user_id": userID})
}

// ---- session endpoints ----

// Me returns the current session info.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u := h.currentUser(r)
	roles := rolesFromContext(r.Context())
	respondJSON(w, http.StatusOK, models.SessionInfo{
		User:          *u,
		EffectiveRole: models.EffectiveRole(roles),
		Roles:         roles,
	})
}

// Logout deletes the session and clears the cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		_ = h.queries.DeleteSession(r.Context(), hashToken(cookie.Value))
	}
	clearCookie(w, sessionCookieName, h.cfg.IsProduction())
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type updateProfileRequest struct {
	DisplayName string `json:"display_name"`
}

// UpdateProfile lets a user set their display name.
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.DisplayName) == "" {
		respondError(w, http.StatusBadRequest, "display_name required")
		return
	}
	u := h.currentUser(r)
	if err := h.queries.UpdateUserDisplayName(r.Context(), u.ID, strings.TrimSpace(req.DisplayName)); err != nil {
		respondError(w, http.StatusInternalServerError, "updating profile")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListMyIdentities returns the current user's linked identities (email decrypted
// for the owner).
func (h *Handler) ListMyIdentities(w http.ResponseWriter, r *http.Request) {
	u := h.currentUser(r)
	rows, err := h.queries.ListUserIdentities(r.Context(), u.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "loading identities")
		return
	}
	out := make([]models.UserIdentity, 0, len(rows))
	for _, row := range rows {
		id := row.Identity()
		ct, wrapped := row.Encrypted()
		if len(ct) > 0 {
			if email, derr := h.encryptor.DecryptPII(&crypto.EncryptedField{Ciphertext: ct, WrappedDEK: wrapped}); derr == nil {
				id.Email = email
			}
		}
		out = append(out, id)
	}
	respondJSON(w, http.StatusOK, out)
}

// ---- invites (preview + accept; claim application lands in Phase 4) ----

// GetInvite previews an invite by token (does not consume it).
func (h *Handler) GetInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := h.queries.GetInviteByTokenHash(r.Context(), hashToken(token))
	if err != nil {
		respondError(w, http.StatusNotFound, "invite not found")
		return
	}
	valid := inv.UsedAt == nil && inv.ExpiresAt.After(time.Now())
	var claim *models.RoleClaim
	if len(inv.ClaimJSON) > 0 {
		claim = &models.RoleClaim{}
		_ = json.Unmarshal(inv.ClaimJSON, claim)
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"kind":       inv.Kind,
		"valid":      valid,
		"expires_at": inv.ExpiresAt,
		"claim":      claim,
	})
}

// AcceptInvite consumes a role_claim invite for the authenticated user. The
// domain-side effect (assigning the contact responsibility) is wired in Phase 4
// once resource_contacts exists; here we validate and mark the invite used.
func (h *Handler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	ctx := r.Context()
	inv, err := h.queries.GetInviteByTokenHash(ctx, hashToken(token))
	if err != nil {
		respondError(w, http.StatusNotFound, "invite not found")
		return
	}
	if inv.UsedAt != nil || inv.ExpiresAt.Before(time.Now()) {
		respondError(w, http.StatusGone, "invite expired or already used")
		return
	}
	u := h.currentUser(r)

	// Apply a role_claim: assign the offered contact responsibility on the
	// resource to the accepting user, linked by their account.
	if inv.Kind == models.InviteRoleClaim && len(inv.ClaimJSON) > 0 {
		var claim models.RoleClaim
		if err := json.Unmarshal(inv.ClaimJSON, &claim); err != nil {
			respondError(w, http.StatusInternalServerError, "invalid claim")
			return
		}
		cilogon := h.primaryCILogonID(ctx, u.ID)
		switch claim.EntityKind {
		case models.KindResource, "":
			resID, err := h.queries.ResourceIDByName(ctx, claim.EntityID)
			if err != nil {
				respondError(w, http.StatusBadRequest, "claim target resource not found")
				return
			}
			if err := h.queries.AddResourceContact(ctx, resID, claim.ContactType, claim.Rank,
				u.DisplayName, cilogon, u.ID); err != nil {
				respondError(w, http.StatusInternalServerError, "assigning responsibility")
				return
			}
		case models.KindResourceGroup, models.KindSite, models.KindFacility:
			if err := h.queries.AddEntityContact(ctx, claim.EntityKind, claim.EntityID,
				claim.ContactType, claim.Rank, u.DisplayName, cilogon, u.ID); err != nil {
				respondError(w, http.StatusInternalServerError, "assigning responsibility")
				return
			}
		default:
			respondError(w, http.StatusBadRequest, "unsupported claim entity kind")
			return
		}
		h.audit(ctx, u.ID, "role_claim.accept", claim.EntityKind, claim.EntityID, "", inv.ClaimJSON)
	}

	if err := h.queries.MarkInviteUsed(ctx, inv.ID, u.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "accepting invite")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// ---- small helpers ----

func (h *Handler) verifyState(r *http.Request, stateParam string) (invite, returnTo string, ok bool) {
	cookie, err := r.Cookie(oidcStateCookie)
	if err != nil {
		return "", "", false
	}
	sig, payloadB64, found := strings.Cut(cookie.Value, ":")
	if !found {
		return "", "", false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return "", "", false
	}
	payload := string(payloadBytes)
	if !crypto.VerifyHMAC(h.sessionSecret, payload, sig) {
		return "", "", false
	}
	parts := strings.SplitN(payload, "|", 3)
	if len(parts) != 3 || parts[0] != stateParam {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func (h *Handler) redirectLogin(w http.ResponseWriter, r *http.Request, errCode string) {
	http.Redirect(w, r, "/login?error="+url.QueryEscape(errCode), http.StatusFound)
}

func (h *Handler) finishRedirect(w http.ResponseWriter, r *http.Request, returnTo, invite string) {
	dest := "/"
	if invite != "" {
		dest = "/invites/accept?token=" + url.QueryEscape(invite)
	} else if returnTo != "" && strings.HasPrefix(returnTo, "/") {
		dest = returnTo
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

func clearCookie(w http.ResponseWriter, name string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: secure, MaxAge: -1,
	})
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// emailSHA1 is the legacy contact id: SHA1 of the lowercased, trimmed email.
func emailSHA1(email string) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])
}

func stringClaim(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
