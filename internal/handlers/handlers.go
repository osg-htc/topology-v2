// Package handlers holds the HTTP handlers, grouped by domain across sibling
// files. The central Handler struct carries shared dependencies and is wired up
// in package router. Small JSON helpers (respondJSON/respondError) are used
// throughout, following the SWAMP/FabAID convention.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/bbockelm/topology-v2/internal/config"
	"github.com/bbockelm/topology-v2/internal/crypto"
	"github.com/bbockelm/topology-v2/internal/db"
	"github.com/bbockelm/topology-v2/internal/version"
)

// Handler carries dependencies shared across all HTTP handlers.
type Handler struct {
	cfg           *config.Config
	queries       *db.Queries
	encryptor     *crypto.Encryptor
	sessionSecret []byte
}

// New constructs a Handler, deriving the encryptor and session secret from the
// instance master key.
func New(cfg *config.Config, queries *db.Queries) (*Handler, error) {
	enc, err := crypto.NewEncryptor(cfg.MasterKey())
	if err != nil {
		return nil, err
	}
	secret, err := crypto.SessionSecret(cfg.MasterKey())
	if err != nil {
		return nil, err
	}
	return &Handler{
		cfg:           cfg,
		queries:       queries,
		encryptor:     enc,
		sessionSecret: secret,
	}, nil
}

// Healthz is a liveness/readiness probe.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": version.Version,
		"commit":  version.Commit,
	})
}

// respondJSON writes v as a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error().Err(err).Msg("encoding JSON response")
	}
}

// respondError writes a JSON error envelope.
func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}
