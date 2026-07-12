-- +goose Up
-- Change-proposal workflow: create/edit/delete never touch live tables directly.
-- A proposal is a living draft (proposed_state JSONB "head") that submitter and
-- managers/admins iterate on via an append-only revision log until it is
-- approved and applied.

CREATE TABLE change_proposals (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_kind    TEXT NOT NULL,          -- resource | resource_group | site | facility | ...
    target_name    TEXT,                   -- entity name for update/delete (null for new registration)
    operation      TEXT NOT NULL,          -- create | update | delete
    proposed_state JSONB NOT NULL DEFAULT '{}'::jsonb,  -- mutable head
    -- Version of the proposed_state JSON Schema this proposal was written
    -- against. The JSONB is decoupled from the live DDL: goose migrates the
    -- live tables, while proposals are brought forward by explicit upgraders
    -- (see internal/proposalschema) so in-flight proposals survive schema drift.
    proposed_schema_version INT NOT NULL DEFAULT 1,
    base_version   JSONB,                  -- snapshot of the live row when branched (conflict detection)
    status         TEXT NOT NULL DEFAULT 'draft', -- draft|pending|approved|rejected|applied|withdrawn
    created_by     UUID NOT NULL REFERENCES users(id),
    assigned_reviewer UUID REFERENCES users(id),
    review_note    TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_proposals_created_by ON change_proposals (created_by);
CREATE INDEX idx_proposals_status ON change_proposals (status);
CREATE INDEX idx_proposals_entity ON change_proposals (entity_kind, target_name);

-- Append-only revision history: every save (by submitter OR reviewer) appends.
CREATE TABLE change_proposal_revisions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    proposal_id    UUID NOT NULL REFERENCES change_proposals(id) ON DELETE CASCADE,
    revision_no    INT NOT NULL,
    proposed_state JSONB NOT NULL,
    edited_by      UUID NOT NULL REFERENCES users(id),
    note           TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (proposal_id, revision_no)
);

-- Immutable audit log. Append-only: no UPDATE/DELETE in application code.
CREATE TABLE audit_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID REFERENCES users(id),  -- null for system actions
    action        TEXT NOT NULL,
    entity_kind   TEXT,
    entity_id     TEXT,
    proposal_id   UUID REFERENCES change_proposals(id),
    detail        JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_created_at ON audit_log (created_at DESC);
CREATE INDEX idx_audit_actor ON audit_log (actor_user_id);
CREATE INDEX idx_audit_entity ON audit_log (entity_kind, entity_id);

-- +goose Down
DROP TABLE audit_log;
DROP TABLE change_proposal_revisions;
DROP TABLE change_proposals;
