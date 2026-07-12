// Package router wires the chi mux: middleware chain, the /api/v1 tree, and the
// SPA fallback that serves the embedded frontend for all non-API routes.
package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"

	"github.com/bbockelm/topology-v2/internal/config"
	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/frontend"
	"github.com/bbockelm/topology-v2/internal/handlers"
	"github.com/bbockelm/topology-v2/internal/storage"
)

// New builds the top-level router and returns it along with the Handler.
func New(cfg *config.Config, queries *db.Queries, store *storage.Store, logger zerolog.Logger) (*chi.Mux, *handlers.Handler, error) {
	h, err := handlers.New(cfg, queries, store)
	if err != nil {
		return nil, nil, err
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(hlog.NewHandler(logger))
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
	}))

	// Health check (unauthenticated).
	r.Get("/healthz", h.Healthz)

	// Versioned API tree. Handlers are added per phase.
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", h.Healthz)

		// Auth (public endpoints).
		r.Route("/auth", func(r chi.Router) {
			r.Get("/mode", h.AuthMode)
			r.Get("/oidc/login", h.OIDCLogin)
			r.Get("/oidc/callback", h.OIDCCallback)
			r.Post("/dev-login", h.DevLogin) // no-op outside development
			r.Post("/logout", h.Logout)

			// Authenticated self-service.
			r.Group(func(r chi.Router) {
				r.Use(h.RequireAuth)
				r.Get("/me", h.Me)
				r.Put("/profile", h.UpdateProfile)
				r.Get("/identities", h.ListMyIdentities)
			})
		})

		// Invite redemption is public (identity is established via OIDC); the
		// accept step requires auth.
		r.Route("/invites", func(r chi.Router) {
			r.Get("/{token}", h.GetInvite) // preview an invite
			r.Group(func(r chi.Router) {
				r.Use(h.RequireAuth)
				r.Post("/{token}/accept", h.AcceptInvite)
			})
		})
	})

	// SPA fallback: everything else serves the embedded frontend.
	spa := frontend.NewSPAHandler()
	r.NotFound(spa.ServeHTTP)

	return r, h, nil
}

// requestLogger emits a structured access-log line per request via zerolog.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		hlog.FromRequest(r).Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.Status()).
			Int("bytes", ww.BytesWritten()).
			Dur("duration", time.Since(start)).
			Msg("request")
	})
}
