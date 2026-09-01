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
	"github.com/bbockelm/topology-v2/internal/models"
)

// New builds the top-level router and returns it along with the Handler.
func New(cfg *config.Config, queries *db.Queries, logger zerolog.Logger) (*chi.Mux, *handlers.Handler, error) {
	h, err := handlers.New(cfg, queries)
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

	// Legacy-compatible topology web API (public, mirrors the existing paths).
	// rgsummary carries contact info, so it uses OptionalAuth: anonymous clients
	// see names only; a signed-in contact_reader sees contact PII.
	r.With(h.OptionalAuth).Get("/rgsummary/xml", h.RGSummaryXML)
	r.Get("/rgdowntime/xml", h.RGDowntimeXML)
	r.Get("/miscresource/json", h.MiscResourceJSON)
	r.Get("/miscsite/json", h.MiscSiteJSON)
	r.Get("/miscfacility/json", h.MiscFacilityJSON)
	r.Get("/vosummary/xml", h.VOSummaryXML)
	r.Get("/vosummary/json", h.VOSummaryJSON)
	r.Get("/miscproject/xml", h.MiscProjectXML)
	r.Get("/miscproject/json", h.MiscProjectJSON)
	r.Get("/schema/{file}", h.ServeSchema)

	// Versioned API tree. Handlers are added per phase.
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", h.Healthz)

		// JSON forms of the topology read API for the frontend (snake_case).
		r.With(h.OptionalAuth).Get("/rgsummary", h.RGSummaryJSON)
		r.Get("/resources", h.ResourcesJSON)
		r.Get("/resources/{id}", h.ResourceDetailHandler)
		r.Get("/summary", h.SummaryHandler)
		r.Get("/resource-groups", h.ListResourceGroupsHandler)
		r.Get("/resource-groups/{name}", h.ResourceGroupDetailHandler)
		r.Get("/sites", h.ListSitesHandler)
		r.Get("/sites/{name}", h.SiteDetailHandler)
		r.Get("/facilities", h.ListFacilitiesHandler)
		r.Get("/facilities/{name}", h.FacilityDetailHandler)
		r.Get("/projects", h.ListProjectsBrowseHandler)
		r.Get("/projects/{name}", h.GetProjectHandler)
		r.Get("/institutions", h.ListInstitutionsHandler)
		r.Get("/downtimes", h.DowntimesHandler)
		// Pick-list sources for form dropdowns.
		r.Get("/service-names", h.ServiceNamesHandler)
		r.Get("/vo-names", h.VONamesHandler)
		r.Get("/tag-names", h.TagsHandler)
		r.Get("/contacts", h.ContactsHandler)

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
		// accept step and invite creation require auth.
		r.Route("/invites", func(r chi.Router) {
			r.Get("/{token}", h.GetInvite) // preview an invite
			r.Group(func(r chi.Router) {
				r.Use(h.RequireAuth)
				r.Post("/", h.CreateInvite)
				r.Post("/{token}/accept", h.AcceptInvite)
			})
		})

		// Authenticated app: dashboard + change-proposal workflow.
		r.Group(func(r chi.Router) {
			r.Use(h.RequireAuth)
			r.Get("/dashboard", h.DashboardHandler)
			r.Get("/user-labels", h.UserLabelsHandler)

			r.Route("/proposals", func(r chi.Router) {
				r.Post("/", h.CreateProposal)
				r.Get("/", h.ListProposalsByEntityHandler)
				r.Get("/mine", h.ListMyProposals)
				r.Get("/pending", h.ListPendingProposals)
				r.Get("/{id}", h.GetProposal)
				r.Put("/{id}", h.ReviseProposal)
				r.Post("/{id}/submit", h.SubmitProposal)
				r.Post("/{id}/withdraw", h.WithdrawProposal)
				r.Post("/{id}/approve", h.ApproveProposal)
				r.Post("/{id}/reject", h.RejectProposal)
			})

			// Contact-replacement requests: propose yourself for a contact slot;
			// the incumbent (or a manager/admin) approves.
			// Pull newly-registered institutions from the registry (rate-limited).
			r.Post("/institutions/refresh", h.RefreshInstitutionsHandler)

			// Email verification: prove control of an address via a single-use link.
			r.Route("/email-verifications", func(r chi.Router) {
				r.Get("/", h.ListEmailVerifications)
				r.Post("/", h.RequestEmailVerification)
				r.Post("/confirm", h.ConfirmEmailVerification)
			})

			r.Route("/contact-replacements", func(r chi.Router) {
				r.Post("/", h.CreateContactReplacement)
				r.Get("/mine", h.ListMyReplacements)
				r.Get("/incoming", h.ListIncomingReplacements)
				r.Post("/{id}/approve", h.ApproveReplacement)
				r.Post("/{id}/reject", h.RejectReplacement)
				r.Post("/{id}/withdraw", h.WithdrawReplacement)
			})

			// Audit log is visible to managers/admins.
			r.With(h.RequireRole(models.RoleManager, models.RoleAdministrator)).
				Get("/audit", h.ListAuditHandler)

			// Administrator-only restore, settings, and user management.
			r.Route("/admin", func(r chi.Router) {
				r.Use(h.RequireRole(models.RoleAdministrator))
				r.Post("/import-github", h.ImportFromGitHub)
				r.Post("/institutions/sync", h.SyncInstitutionsHandler)
				r.Get("/oidc-config", h.GetOIDCConfig)
				r.Put("/oidc-config", h.SetOIDCConfig)
				r.Get("/users", h.ListUsersHandler)
				r.Get("/users/search", h.SearchUsersHandler)
				r.Post("/users/{id}/roles", h.SetUserRoleHandler)
				r.Post("/users/{id}/username", h.SetUsernameHandler)
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
