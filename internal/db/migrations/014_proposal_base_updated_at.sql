-- +goose Up
-- TEMPORARY: dev-only optimistic-concurrency guard (see
-- internal/handlers/proposal_stale.go). Records the target entity's
-- updated_at at the moment a proposal's base_version was snapshotted, so an
-- apply can be refused if the live entity has since changed underneath it.
-- Drop this column when that file is removed in favor of a real
-- concurrency/merge design.
ALTER TABLE change_proposals ADD COLUMN base_updated_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE change_proposals DROP COLUMN base_updated_at;
