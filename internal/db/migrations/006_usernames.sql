-- +goose Up
-- Every user gets a unique username (derived at first login, admin-changeable),
-- alongside the self-changeable display_name and the immutable id. Nullable so
-- existing rows and provisioned contacts (no login yet) are valid; a partial
-- unique index enforces uniqueness among set usernames.
ALTER TABLE users ADD COLUMN username TEXT;
CREATE UNIQUE INDEX idx_users_username ON users (username) WHERE username IS NOT NULL;

-- +goose Down
DROP INDEX idx_users_username;
ALTER TABLE users DROP COLUMN username;
