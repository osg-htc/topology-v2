-- +goose Up
-- Campus grid sponsor IDs: a shared registry (projects/_CAMPUS_GRIDS.yaml)
-- mapping a campus grid's name to its explicit legacy ID, cross-referenced
-- by a project's Sponsor.CampusGrid.Name. Pure reference/lookup data, not a
-- user-editable domain entity -- upsert-by-name like institutions/support_centers.
CREATE TABLE campus_grids (
    name           TEXT PRIMARY KEY,
    campus_grid_id BIGINT NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE campus_grids;
