-- +goose Up
-- Email verification: a user proves control of an email address by clicking a
-- single-use link. The plaintext address is never stored — only its SHA1 lookup
-- hash, a masked hint for display, and the envelope-encrypted ciphertext (so a
-- confirmed address can later be surfaced to the owner). One row per
-- (user, email); re-requesting regenerates the token and clears verified_at.
CREATE TABLE email_verifications (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email_sha1        TEXT NOT NULL,
    email_hint        TEXT NOT NULL,           -- masked, e.g. j***@example.org
    email_ciphertext  BYTEA,
    email_dek_wrapped BYTEA,
    token_hash        BYTEA NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at        TIMESTAMPTZ NOT NULL,
    verified_at       TIMESTAMPTZ,
    UNIQUE (user_id, email_sha1)
);
CREATE INDEX idx_email_verifications_token ON email_verifications (token_hash)
    WHERE verified_at IS NULL;

-- +goose Down
DROP TABLE email_verifications;
