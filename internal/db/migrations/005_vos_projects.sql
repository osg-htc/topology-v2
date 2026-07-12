-- +goose Up
-- Virtual organizations and projects. Unlike the resource hierarchy (which is
-- modeled relationally for querying), these are stored as lossless documents:
-- the raw YAML is preserved verbatim for a byte-exact backup/restore round-trip,
-- with a few indexed columns extracted for lookups and the API.

CREATE TABLE vos (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,            -- VO file name (minus .yaml)
    vo_id      BIGINT NOT NULL,          -- ID field (gen_id fallback)
    disable    BOOLEAN NOT NULL DEFAULT FALSE,
    raw_yaml   TEXT NOT NULL,            -- verbatim source for round-trip
    deleted_at TIMESTAMPTZ,
    deleted_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_vos_name_active ON vos (name) WHERE deleted_at IS NULL;

-- Projects are heavily used, so they are modeled relationally with typed
-- columns. Sponsor (a small polymorphic CampusGrid|VirtualOrganization block)
-- and any other/complex fields (e.g. ResourceAllocations) use JSONB, with an
-- `extra` inline catch-all guaranteeing lossless round-trip.
CREATE TABLE projects (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name               TEXT NOT NULL,       -- project file name (minus .yaml)
    project_id         TEXT,                -- ID field (string in YAML)
    description        TEXT,
    department         TEXT,
    field_of_science   TEXT,
    field_of_science_id TEXT,
    organization       TEXT,
    pi_name            TEXT,
    institution_id     TEXT,
    sponsor            JSONB,               -- {CampusGrid|VirtualOrganization: {...}}
    sponsor_type       TEXT,                -- derived, for querying
    sponsor_name       TEXT,                -- derived, for querying
    extra              JSONB,               -- lossless catch-all (ResourceAllocations, etc.)
    deleted_at         TIMESTAMPTZ,
    deleted_by         UUID REFERENCES users(id),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_projects_name_active ON projects (name) WHERE deleted_at IS NULL;
CREATE INDEX idx_projects_sponsor ON projects (sponsor_type, sponsor_name) WHERE deleted_at IS NULL;
CREATE INDEX idx_projects_fos ON projects (field_of_science) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE projects;
DROP TABLE vos;
