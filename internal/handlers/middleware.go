package handlers

import (
	"context"
	"crypto/sha256"
	"net/http"

	"github.com/bbockelm/topology-v2/internal/models"
)

// ctxKey is a private context key type.
type ctxKey string

const (
	ctxSession ctxKey = "session"
	ctxUser    ctxKey = "user"
	ctxRoles   ctxKey = "roles"
)

const sessionCookieName = "topology_session"

// hashToken returns the SHA-256 of a raw bearer token. High-entropy tokens
// (256 bits) need no slow hash — SHA-256 is used for sessions and invites.
func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

// RequireAuth validates the session cookie, loads the session + user, and
// injects them into the request context.
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			respondError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		ctx := r.Context()
		session, err := h.queries.GetSessionByTokenHash(ctx, hashToken(cookie.Value))
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid session")
			return
		}
		user, err := h.queries.GetUser(ctx, session.UserID)
		if err != nil || user.Status != "active" {
			respondError(w, http.StatusUnauthorized, "account not active")
			return
		}
		roles, err := h.queries.GetUserRoles(ctx, user.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "loading roles")
			return
		}
		if len(roles) == 0 {
			roles = []string{models.RoleUser}
		}
		user.Roles = roles

		ctx = context.WithValue(ctx, ctxSession, session)
		ctx = context.WithValue(ctx, ctxUser, user)
		ctx = context.WithValue(ctx, ctxRoles, roles)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole returns middleware that permits only sessions whose effective
// role is one of the allowed roles. Administrator always passes.
func (h *Handler) RequireRole(allowed ...string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		allowedSet[a] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			roles := rolesFromContext(r.Context())
			eff := models.EffectiveRole(roles)
			if eff == models.RoleAdministrator || allowedSet[eff] {
				next.ServeHTTP(w, r)
				return
			}
			respondError(w, http.StatusForbidden, "insufficient role")
		})
	}
}

// ---- context accessors ----

func sessionFromContext(ctx context.Context) *models.Session {
	s, _ := ctx.Value(ctxSession).(*models.Session)
	return s
}

func userFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(ctxUser).(*models.User)
	return u
}

func rolesFromContext(ctx context.Context) []string {
	r, _ := ctx.Value(ctxRoles).([]string)
	return r
}

// currentUser is a convenience for handlers.
func (h *Handler) currentUser(r *http.Request) *models.User {
	return userFromContext(r.Context())
}
