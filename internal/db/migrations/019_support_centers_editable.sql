-- +goose Up
-- Support centers become editable through the change-proposal workflow, so they
-- need what every other domain entity already has:
--   * soft-delete (deleted_at/deleted_by), with the name uniqueness narrowed to
--     live rows so a deleted center's name can be reused;
--   * updated_at, for the stale-base guard;
--   * extra, a lossless catch-all. support-centers.yaml entries carry a
--     Contacts block (40 of 52 real centers) that the importer used to drop on
--     the floor -- and with it, from every backup/restore round trip.
ALTER TABLE support_centers
    ADD COLUMN extra      JSONB,
    ADD COLUMN deleted_at TIMESTAMPTZ,
    ADD COLUMN deleted_by UUID REFERENCES users(id),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE support_centers DROP CONSTRAINT support_centers_name_key;
CREATE UNIQUE INDEX idx_support_centers_name_active ON support_centers (name) WHERE deleted_at IS NULL;

-- +goose Down
DELETE FROM support_centers WHERE deleted_at IS NOT NULL;
DROP INDEX idx_support_centers_name_active;
ALTER TABLE support_centers ADD CONSTRAINT support_centers_name_key UNIQUE (name);
ALTER TABLE support_centers
    DROP COLUMN updated_at,
    DROP COLUMN deleted_by,
    DROP COLUMN deleted_at,
    DROP COLUMN extra;
