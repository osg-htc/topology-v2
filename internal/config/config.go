// Package config loads server configuration from the environment (via envconfig)
// and bootstraps the instance master key used for envelope encryption.
//
// The design mirrors the SWAMP/FabAID pattern: a single Config struct populated
// from env vars, a ValidateServer() gate for required fields, and a master key
// that is either supplied (INSTANCE_KEY) or auto-generated and persisted on
// first run. From that one master key, HKDF derives every subordinate key
// (session HMAC secret, PII KEK, backup keys) — no external KMS is required.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all runtime configuration. Field tags map to TOPOLOGY_* env vars.
type Config struct {
	// General
	AppEnv  string `envconfig:"APP_ENV" default:"development"` // development | production
	Port    string `envconfig:"PORT" default:"8080"`
	BaseURL string `envconfig:"BASE_URL" default:"http://localhost:8080"`

	// Database
	DatabaseURL string `envconfig:"DATABASE_URL"`

	// Instance master key (32 bytes hex = 64 chars). If empty, auto-generated
	// and written to MasterKeyPath on first run.
	InstanceKey   string `envconfig:"INSTANCE_KEY"`
	MasterKeyPath string `envconfig:"MASTER_KEY_PATH" default:".topology-master.key"`

	// OIDC (CILogon by default). May be overridden at runtime via app_config.
	OIDCIssuer       string `envconfig:"OIDC_ISSUER" default:"https://cilogon.org"`
	OIDCClientID     string `envconfig:"OIDC_CLIENT_ID"`
	OIDCClientSecret string `envconfig:"OIDC_CLIENT_SECRET"`

	// External institutions registry (OSG IID <-> ROR). Failures are soft:
	// institution IDs are immutable, so we cache aggressively in the DB.
	InstitutionsAPI string `envconfig:"INSTITUTIONS_API" default:"https://topology-institutions.osg-htc.org/api"`

	// GitHub topology repo (the admin "Import from GitHub" restore action).
	GitHubRepo  string `envconfig:"GITHUB_REPO" default:"opensciencegrid/topology"`
	GitHubToken string `envconfig:"GITHUB_TOKEN"`

	// masterKey is the decoded 32-byte key; populated by EnsureMasterKey.
	masterKey []byte
}

// Load reads configuration from the environment.
func Load() (*Config, error) {
	var c Config
	if err := envconfig.Process("TOPOLOGY", &c); err != nil {
		return nil, fmt.Errorf("processing env config: %w", err)
	}
	return &c, nil
}

// IsProduction reports whether the app is running in production mode.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

// ValidateServer enforces the fields required to run the HTTP server.
func (c *Config) ValidateServer() error {
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.IsProduction() {
		if c.OIDCClientID == "" {
			missing = append(missing, "OIDC_CLIENT_ID")
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	return nil
}

// EnsureMasterKey loads the 32-byte instance master key from INSTANCE_KEY, or
// from MasterKeyPath, or generates and persists a new one (mode 0600).
func (c *Config) EnsureMasterKey() error {
	if c.InstanceKey != "" {
		key, err := hex.DecodeString(strings.TrimSpace(c.InstanceKey))
		if err != nil || len(key) != 32 {
			return fmt.Errorf("INSTANCE_KEY must be 64 hex chars (32 bytes)")
		}
		c.masterKey = key
		return nil
	}

	// Try reading a previously persisted key.
	if data, err := os.ReadFile(c.MasterKeyPath); err == nil {
		key, derr := hex.DecodeString(strings.TrimSpace(string(data)))
		if derr == nil && len(key) == 32 {
			c.masterKey = key
			return nil
		}
	}

	// Generate a fresh key and persist it.
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("generating master key: %w", err)
	}
	if err := os.WriteFile(c.MasterKeyPath, []byte(hex.EncodeToString(key)), 0o600); err != nil {
		return fmt.Errorf("persisting master key to %s: %w", c.MasterKeyPath, err)
	}
	c.masterKey = key
	return nil
}

// MasterKey returns the decoded 32-byte master key. Call EnsureMasterKey first.
func (c *Config) MasterKey() []byte {
	return c.masterKey
}
