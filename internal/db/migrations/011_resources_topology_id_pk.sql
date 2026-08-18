-- +goose Up
-- Re-key resources from an internal UUID to the legacy topology_id integer.
-- topology_id becomes the sole, immutable identity for a resource: renaming a
-- resource must never change it, and it is now a real foreign-key target for
-- resource_services, resource_contacts, and downtimes.
--
-- This also retires soft-delete+insert as an update mechanism for resources
-- (a Phase-4 code-reuse shortcut, never a real versioning feature -- zero rows
-- have ever actually been soft-deleted on any table in this system). A hard
-- PRIMARY KEY cannot tolerate a tombstoned row and a live row sharing a key, so
-- resource edits become a real in-place UPDATE, matching the pattern already
-- used by sites/resource_groups/facilities. deleted_at/deleted_by stay, used
-- only for genuine delete operations, same as everywhere else.

-- 1. Drop the two FKs pointing at resources(id) so the columns can be retargeted.
ALTER TABLE resource_services DROP CONSTRAINT resource_services_resource_id_fkey;
ALTER TABLE resource_contacts DROP CONSTRAINT resource_contacts_resource_id_fkey;

-- 2. Swap resource_services.resource_id and resource_contacts.resource_id from
--    UUID to BIGINT, backfilled via the still-present UUID join.
ALTER TABLE resource_services ADD COLUMN resource_topology_id BIGINT;
UPDATE resource_services rs SET resource_topology_id = r.topology_id
  FROM resources r WHERE r.id = rs.resource_id;
ALTER TABLE resource_services ALTER COLUMN resource_topology_id SET NOT NULL;
ALTER TABLE resource_services DROP COLUMN resource_id;
ALTER TABLE resource_services RENAME COLUMN resource_topology_id TO resource_id;

ALTER TABLE resource_contacts ADD COLUMN resource_topology_id BIGINT;
UPDATE resource_contacts rc SET resource_topology_id = r.topology_id
  FROM resources r WHERE r.id = rc.resource_id;
ALTER TABLE resource_contacts ALTER COLUMN resource_topology_id SET NOT NULL;
ALTER TABLE resource_contacts DROP COLUMN resource_id;
ALTER TABLE resource_contacts RENAME COLUMN resource_topology_id TO resource_id;

-- 3. Drop the UUID primary key and column; promote topology_id.
ALTER TABLE resources DROP CONSTRAINT resources_pkey;
ALTER TABLE resources DROP COLUMN id;
ALTER TABLE resources ADD PRIMARY KEY (topology_id);

-- 4. Re-point the two FKs at the new key.
ALTER TABLE resource_services ADD CONSTRAINT resource_services_resource_id_fkey
  FOREIGN KEY (resource_id) REFERENCES resources(topology_id) ON DELETE CASCADE;
ALTER TABLE resource_contacts ADD CONSTRAINT resource_contacts_resource_id_fkey
  FOREIGN KEY (resource_id) REFERENCES resources(topology_id) ON DELETE CASCADE;
CREATE INDEX idx_resource_services_resource ON resource_services (resource_id);
CREATE INDEX idx_resource_contacts_resource ON resource_contacts (resource_id);

-- 5. Give downtimes a real FK alongside resource_name (kept: /rgdowntime/xml
--    renders the resource name, not the id, and always will). Nullable: a
--    downtime whose ResourceName doesn't resolve to a live resource is a
--    known, tolerated case (v1 and v2 both already produce a broken-but-
--    non-fatal entry for it rather than rejecting the whole batch) -- a
--    NOT NULL FK would turn one dangling reference into a hard failure of
--    the entire import, a new and worse failure mode than today's.
ALTER TABLE downtimes ADD COLUMN resource_id BIGINT;
UPDATE downtimes d SET resource_id = r.topology_id
  FROM resources r WHERE r.name = d.resource_name AND r.deleted_at IS NULL;
ALTER TABLE downtimes ADD CONSTRAINT downtimes_resource_id_fkey
  FOREIGN KEY (resource_id) REFERENCES resources(topology_id);
CREATE INDEX idx_downtimes_resource ON downtimes (resource_id);

-- 6. Sequence for app-created resources with no explicit ID. Seeded one past
--    the highest ID a human has ever actually written into a YAML file --
--    reimplements bin/next_resource_id's own algorithm (scan current max,
--    +1) as a safe atomic mechanism. Never touches the import-time hash
--    fallback (1 + md5(name) mod (2^31-1)), which stays name-derived and
--    deterministic, unmanaged by this sequence.
CREATE SEQUENCE resources_app_created_id_seq;
SELECT setval('resources_app_created_id_seq',
  (SELECT COALESCE(MAX(topology_id), 0) FROM resources WHERE id_explicit) + 1, false);

-- +goose Down
DROP SEQUENCE resources_app_created_id_seq;
ALTER TABLE downtimes DROP CONSTRAINT downtimes_resource_id_fkey;
DROP INDEX idx_downtimes_resource;
ALTER TABLE downtimes DROP COLUMN resource_id;
ALTER TABLE resource_contacts DROP CONSTRAINT resource_contacts_resource_id_fkey;
ALTER TABLE resource_services DROP CONSTRAINT resource_services_resource_id_fkey;
DROP INDEX idx_resource_contacts_resource;
DROP INDEX idx_resource_services_resource;
ALTER TABLE resources DROP CONSTRAINT resources_pkey;
ALTER TABLE resources ADD COLUMN id UUID DEFAULT gen_random_uuid() NOT NULL;
ALTER TABLE resources ADD PRIMARY KEY (id);
-- Regenerating resource_services/resource_contacts UUID linkage below is
-- necessarily lossy (fresh UUIDs, not the originals) -- inherent to any down
-- migration that changes a table's identity scheme, not specific to this one.
ALTER TABLE resource_services ADD COLUMN resource_id_new UUID;
UPDATE resource_services rs SET resource_id_new = r.id
  FROM resources r WHERE r.topology_id = rs.resource_id;
ALTER TABLE resource_services DROP COLUMN resource_id;
ALTER TABLE resource_services RENAME COLUMN resource_id_new TO resource_id;
ALTER TABLE resource_services ALTER COLUMN resource_id SET NOT NULL;
ALTER TABLE resource_services ADD CONSTRAINT resource_services_resource_id_fkey
  FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE;
CREATE INDEX idx_resource_services_resource ON resource_services (resource_id);

ALTER TABLE resource_contacts ADD COLUMN resource_id_new UUID;
UPDATE resource_contacts rc SET resource_id_new = r.id
  FROM resources r WHERE r.topology_id = rc.resource_id;
ALTER TABLE resource_contacts DROP COLUMN resource_id;
ALTER TABLE resource_contacts RENAME COLUMN resource_id_new TO resource_id;
ALTER TABLE resource_contacts ALTER COLUMN resource_id SET NOT NULL;
ALTER TABLE resource_contacts ADD CONSTRAINT resource_contacts_resource_id_fkey
  FOREIGN KEY (resource_id) REFERENCES resources(id) ON DELETE CASCADE;
CREATE INDEX idx_resource_contacts_resource ON resource_contacts (resource_id);
