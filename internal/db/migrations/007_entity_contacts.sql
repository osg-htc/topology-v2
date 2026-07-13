-- +goose Up
-- Contacts on resource groups / sites / facilities, so resources can inherit
-- them. Keyed by (entity_kind, entity_name) — names are unique among active
-- entities of a kind. Resource contacts stay in resource_contacts; this table
-- holds the parent-level contacts that resources inherit unless they override a
-- given contact type. Contacts must be users (user_id), like resource_contacts.
CREATE TABLE entity_contacts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_kind  TEXT NOT NULL,   -- resource_group | site | facility
    entity_name  TEXT NOT NULL,
    contact_type TEXT NOT NULL,
    rank         TEXT NOT NULL,
    contact_name TEXT,
    contact_id   TEXT,
    user_id      UUID REFERENCES users(id),
    deleted_at   TIMESTAMPTZ,
    deleted_by   UUID REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_entity_contacts_lookup ON entity_contacts (entity_kind, entity_name)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE entity_contacts;
