-- +goose Up
-- Authentication & authorization, modeled on FabAID: one account (users) with
-- many federated identities (user_identities), opaque hashed sessions, hashed
-- single-use invites, multi-role assignment, and hashed API keys.

CREATE TABLE users (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'active',   -- active | disabled | provisioned
    -- Legacy contact identity carried over from the YAML topology data model.
    -- One of: OSG id (e.g. OSG1000016) or SHA1(lowercased email). Nullable for
    -- accounts created fresh in this app.
    legacy_contact_id TEXT,
    is_provisioned BOOLEAN NOT NULL DEFAULT FALSE,  -- true = contact stub, never logged in
    last_login   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_users_legacy_contact_id ON users (legacy_contact_id)
    WHERE legacy_contact_id IS NOT NULL;

-- Federated identities. UNIQUE(issuer, subject) guarantees an identity maps to
-- exactly one account; a 23505 on insert means "already linked elsewhere."
CREATE TABLE user_identities (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer      TEXT NOT NULL,
    subject     TEXT NOT NULL,
    -- Email is PII: stored as envelope-encrypted ciphertext + wrapped DEK.
    -- email_sha1 is the legacy contact-id hash (SHA1 of lowercased email) and
    -- doubles as a non-reversible lookup key. Plaintext is never stored.
    email_ciphertext  BYTEA,
    email_dek_wrapped BYTEA,
    email_sha1        TEXT,
    eppn         TEXT,
    oidc         TEXT,
    cilogon_id   TEXT,
    idp_name     TEXT,
    display_name TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (issuer, subject)
);
CREATE INDEX idx_user_identities_user_id ON user_identities (user_id);
CREATE INDEX idx_user_identities_email_sha1 ON user_identities (email_sha1)
    WHERE email_sha1 IS NOT NULL;
CREATE INDEX idx_user_identities_cilogon_id ON user_identities (cilogon_id)
    WHERE cilogon_id IS NOT NULL;

-- Opaque sessions: only the SHA-256 hash of the raw token is stored.
CREATE TABLE sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,           -- effective role snapshotted at login
    token_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_sessions_token_hash ON sessions (token_hash);
CREATE INDEX idx_sessions_user_id ON sessions (user_id);

-- Roles: administrator | manager | user. Multiple per user; a session carries
-- the highest effective role.
CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role    TEXT NOT NULL,
    PRIMARY KEY (user_id, role)
);

-- Invites: single-use, hashed, expiring. Two kinds:
--   account_link : link a new federated identity to target_user_id (FabAID).
--   role_claim   : grant the invitee a responsibility described by `claim`
--                  (e.g. Security Contact on a resource). target_user_id may be
--                  null (invitee onboards as themselves).
CREATE TABLE invites (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind           TEXT NOT NULL,                 -- account_link | role_claim
    token_hash     BYTEA NOT NULL,
    created_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    target_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    claim          JSONB,                         -- role_claim payload
    used_at        TIMESTAMPTZ,
    used_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at     TIMESTAMPTZ NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_invites_token_hash ON invites (token_hash);

-- API keys: bcrypt-hashed, displayable prefix, multi-role, soft-revocable.
CREATE TABLE api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL DEFAULT '',
    key_prefix   TEXT NOT NULL,
    key_hash     TEXT NOT NULL,
    roles        TEXT[] NOT NULL DEFAULT '{}',
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_api_keys_user_id ON api_keys (user_id);
CREATE INDEX idx_api_keys_prefix ON api_keys (key_prefix);

-- +goose Down
DROP TABLE api_keys;
DROP TABLE invites;
DROP TABLE user_roles;
DROP TABLE sessions;
DROP TABLE user_identities;
DROP TABLE users;
