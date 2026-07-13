-- +goose Up
-- Requests to take over a specific contact slot from its current holder. A slot
-- is identified by (entity_kind, entity_name, contact_type, rank). The request
-- can be approved either by the incumbent being replaced (a person may hand off
-- their own responsibility) or by a manager/administrator. On approval the slot
-- is repointed to the requester.
CREATE TABLE contact_replacements (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_kind        TEXT NOT NULL,   -- resource | resource_group | site | facility
    entity_name        TEXT NOT NULL,
    contact_type       TEXT NOT NULL,
    rank               TEXT NOT NULL,
    incumbent_user_id  UUID REFERENCES users(id),
    incumbent_name     TEXT,
    requester_user_id  UUID NOT NULL REFERENCES users(id),
    requester_name     TEXT,
    requester_contact_id TEXT,
    status             TEXT NOT NULL DEFAULT 'pending', -- pending|approved|rejected|withdrawn
    note               TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    decided_at         TIMESTAMPTZ,
    decided_by         UUID REFERENCES users(id)
);
CREATE INDEX idx_contact_replacements_incumbent ON contact_replacements (incumbent_user_id)
    WHERE status = 'pending';
CREATE INDEX idx_contact_replacements_requester ON contact_replacements (requester_user_id);

-- +goose Down
DROP TABLE contact_replacements;
