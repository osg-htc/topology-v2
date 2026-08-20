-- +goose Up
-- Reporting groups: a shared registry cross-referenced by VOs (a VO's
-- ReportingGroups field is just a list of names into this table), imported
-- from virtual-organizations/REPORTING_GROUPS.yaml. Pure reference/lookup
-- data, not a user-editable domain entity -- no soft-delete, upsert-by-name
-- like the institutions/support_centers caches.
CREATE TABLE reporting_groups (
    name       TEXT PRIMARY KEY,
    contacts   JSONB,
    fqans      JSONB,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE reporting_groups;
