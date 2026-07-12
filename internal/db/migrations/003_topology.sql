-- +goose Up
-- Core topology domain. Resources are the primary entity; resource_groups →
-- sites → facilities → institutions form the secondary hierarchy. Everything is
-- soft-deleted (deleted_at/deleted_by) with active-row uniqueness enforced by
-- partial unique indexes.

-- Institutions: cached from the external registry (OSG IID <-> ROR). Because
-- institution ids are immutable, the cache is authoritative on API failure.
CREATE TABLE institutions (
    iid_uri    TEXT PRIMARY KEY,        -- https://osg-htc.org/iid/...
    name       TEXT NOT NULL,
    ror_id     TEXT,
    cached_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE facilities (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topology_id    BIGINT NOT NULL,          -- explicit or gen_id-derived
    name           TEXT NOT NULL,            -- directory name in the YAML tree
    institution_id TEXT,                     -- InstitutionID URI (may be null)
    extra          JSONB,                    -- lossless catch-all for unmodeled keys
    id_explicit    BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at     TIMESTAMPTZ,
    deleted_by     UUID REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_facilities_name_active ON facilities (name) WHERE deleted_at IS NULL;

CREATE TABLE sites (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topology_id   BIGINT NOT NULL,
    facility_id   UUID NOT NULL REFERENCES facilities(id),
    name          TEXT NOT NULL,
    long_name     TEXT,
    description   TEXT,
    address_line1 TEXT,
    address_line2 TEXT,
    city          TEXT,
    state         TEXT,
    country       TEXT,
    zipcode       TEXT,                       -- string: leading zeros matter
    latitude      DOUBLE PRECISION,
    longitude     DOUBLE PRECISION,
    extra         JSONB,
    id_explicit   BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at    TIMESTAMPTZ,
    deleted_by    UUID REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_sites_name_active ON sites (name) WHERE deleted_at IS NULL;

CREATE TABLE resource_groups (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id          BIGINT NOT NULL,        -- GroupID in YAML
    site_id           UUID NOT NULL REFERENCES sites(id),
    name              TEXT NOT NULL,          -- RG file name (minus .yaml)
    production        BOOLEAN,                -- nullable: preserve source absence
    support_center    TEXT,
    group_description TEXT,
    extra             JSONB,
    id_explicit       BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at        TIMESTAMPTZ,
    deleted_by        UUID REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_rgs_name_active ON resource_groups (name) WHERE deleted_at IS NULL;

-- Resources: the primary entity. Irregular leaf structures (VO ownership, WLCG
-- info) are kept as JSONB for lossless round-trip; scalar/array fields are typed
-- columns for querying.
CREATE TABLE resources (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topology_id       BIGINT NOT NULL,
    resource_group_id UUID NOT NULL REFERENCES resource_groups(id),
    name              TEXT NOT NULL,
    active            BOOLEAN,                -- nullable: preserve source absence
    description       TEXT,
    fqdn              TEXT NOT NULL,
    dn                TEXT,
    fqdn_aliases      TEXT[],
    tags              TEXT[],
    allowed_vos       TEXT[],
    vo_ownership      JSONB,                  -- {VO: percent}
    wlcg_information  JSONB,
    extra             JSONB,
    id_explicit       BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at        TIMESTAMPTZ,
    deleted_by        UUID REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_resources_name_active ON resources (name) WHERE deleted_at IS NULL;
CREATE INDEX idx_resources_rg ON resources (resource_group_id);

-- One row per service offered by a resource. service_name references
-- services.name; details holds the optional Details map.
CREATE TABLE resource_services (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id UUID NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    service_name TEXT NOT NULL,
    description TEXT,
    details     JSONB,
    ordinal     INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_resource_services_resource ON resource_services (resource_id);

-- Contact assignments on a resource. contact_id is the legacy id (OSG id or
-- SHA1-of-email); user_id links to the internalized account when resolvable.
CREATE TABLE resource_contacts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id  UUID NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    contact_type TEXT NOT NULL,     -- Administrative Contact | Security Contact | ...
    rank         TEXT NOT NULL,     -- Primary | Secondary | Tertiary
    contact_name TEXT,
    contact_id   TEXT,              -- legacy id from the YAML
    user_id      UUID REFERENCES users(id),
    deleted_at   TIMESTAMPTZ,
    deleted_by   UUID REFERENCES users(id)
);
CREATE INDEX idx_resource_contacts_resource ON resource_contacts (resource_id);
CREATE INDEX idx_resource_contacts_user ON resource_contacts (user_id) WHERE user_id IS NOT NULL;

CREATE TABLE downtimes (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dt_id             BIGINT NOT NULL,        -- ID in YAML (epoch-based)
    resource_group_id UUID NOT NULL REFERENCES resource_groups(id),
    resource_name     TEXT NOT NULL,
    class             TEXT NOT NULL,          -- SCHEDULED | UNSCHEDULED
    severity          TEXT,
    description       TEXT,
    start_time        TEXT NOT NULL,          -- kept as source string for fidelity
    end_time          TEXT NOT NULL,
    created_time      TEXT,
    services          TEXT[],
    ordinal           INT NOT NULL DEFAULT 0,  -- preserve file order within an RG
    deleted_at        TIMESTAMPTZ,
    deleted_by        UUID REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_downtimes_rg ON downtimes (resource_group_id);

-- services.yaml: "<Service Name>": <int id>
CREATE TABLE services (
    id   BIGINT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

-- support-centers.yaml
CREATE TABLE support_centers (
    id          BIGINT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    long_name   TEXT,
    community   TEXT,
    description TEXT
);

-- +goose Down
DROP TABLE support_centers;
DROP TABLE services;
DROP TABLE downtimes;
DROP TABLE resource_contacts;
DROP TABLE resource_services;
DROP TABLE resources;
DROP TABLE resource_groups;
DROP TABLE sites;
DROP TABLE facilities;
DROP TABLE institutions;
