-- +goose Up
-- The temporary stale-base guard (migration 014) is replaced by a real
-- three-way merge at apply time (see internal/handlers/proposal_merge3.go),
-- which needs no stored updated_at snapshot.
ALTER TABLE change_proposals DROP COLUMN base_updated_at;

-- +goose Down
ALTER TABLE change_proposals ADD COLUMN base_updated_at TIMESTAMPTZ;
