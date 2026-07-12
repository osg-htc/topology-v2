-- +goose Up
-- Core extensions used across the schema: pgcrypto for gen_random_uuid().
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- app_config: admin-editable runtime configuration that overrides env vars at
-- boot (e.g. OIDC issuer/client-id, and encrypted secrets). Mirrors FabAID.
CREATE TABLE app_config (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE app_config;
DROP EXTENSION IF EXISTS pgcrypto;
