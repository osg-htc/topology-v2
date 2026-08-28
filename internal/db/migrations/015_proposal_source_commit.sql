-- +goose Up
-- Marks a change_proposals row as replayed from a specific historical git
-- commit (see internal/githistory), rather than created through the normal
-- review workflow. NULL for every ordinary proposal. The partial unique
-- index makes re-running the history importer idempotent: a commit already
-- imported is simply skipped.
ALTER TABLE change_proposals ADD COLUMN source_commit_sha TEXT;
CREATE UNIQUE INDEX idx_change_proposals_source_commit_sha
    ON change_proposals (source_commit_sha) WHERE source_commit_sha IS NOT NULL;

-- +goose Down
DROP INDEX idx_change_proposals_source_commit_sha;
ALTER TABLE change_proposals DROP COLUMN source_commit_sha;
