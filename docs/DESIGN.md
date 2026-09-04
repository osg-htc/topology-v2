# Topology Webapp — Design

A ground-up rewrite of the OSG topology registry as a Go + Postgres webapp,
replacing the hand-edited YAML + GitHub-PR workflow. The stack and patterns are
modeled on the SWAMP and FabAID apps.

## Goals

- **Resources are the primary entity.** Institutions, sites, and resource groups
  are the secondary hierarchy that resources hang from.
- **Faithful compatibility** with the existing topology data model and web API
  (byte-compatible XML for `/rgsummary/xml`, `/rgdowntime/xml`, `/vosummary/xml`,
  `/misc*`, etc.), plus a modern JSON API for the new frontend.
- **Restore round-trip** with the existing topology GitHub repo (modulo
  whitespace) until an official switchover. Database backups themselves are
  handled at the infrastructure layer (Postgres's own backup mechanism), not
  by this app.
- **Copy FabAID's authorization**, especially federated identity linking.
- **Soft-delete only.** Nothing is hard-deleted.
- **Propose-then-approve** change flow: creation/edit produce proposals, editable
  and overridable by `manager`/`administrator`.
- **Encrypt emails** (and other PII) via per-row wrapped Data Encryption Keys.

## Stack

- Go 1.26, `go-chi/chi` router, `jackc/pgx/v5` pool with a hand-written `Queries`
  layer (no ORM), `pressly/goose` migrations embedded via `go:embed` and run at
  boot, `kelseyhightower/envconfig`, `rs/zerolog`.
- Next.js (App Router) + React + Tailwind (`brand`/`navy` palette), React Query,
  a `fetchJSON` API client. Production: static export → `go:embed` → single binary.

## Authentication & authorization (from FabAID)

- `users` ⇄ `user_identities` with `UNIQUE(issuer, subject)`. One account, many
  federated identities. A Postgres `23505` on identity insert means "already
  linked to a different account."
- Sessions are opaque 256-bit tokens; only the SHA-256 hash is stored. Cookie is
  `HttpOnly, SameSite=Lax`.
- OIDC Authorization Code flow (CILogon by default). CSRF state is HMAC-signed
  with a key derived from the master key and carried in a short-lived cookie; an
  invite token can ride along inside the signed state.
- Dev mode: a dev-login endpoint bypasses OIDC.
- **Roles:** `administrator`, `manager`, `user` (multiple per user; a session
  carries the highest effective role). Layered chi middleware groups enforce
  coarse role checks; resource-level checks enforce ownership/contact scope.
  Only `administrator` may back up / restore.

## Encryption (envelope, from FabAID)

One 32-byte instance master key (env `INSTANCE_KEY` or auto-generated file).
HKDF-SHA256 derives domain-separated subkeys by `info` label:

- `topology-session-secret` → OIDC-state HMAC key.
- `topology-pii-kek` → KEK that wraps per-row PII DEKs.
- `topology-backup-key` → base for per-backup keys.

Email/PII columns are stored as: `*_ciphertext`, `*_dek_wrapped`, `*_dek_nonce`
(all AES-256-GCM). Alongside the encrypted email we also store the **legacy
contact ID** (`SHA1(lowercased trimmed email)`), used by the legacy data model,
and a lookup hash for dedup — the plaintext email never sits in a column.

## Domain model

Primary: **`resources`** (`fqdn`, `active`, `description`, services, tags,
allowed-VOs, VO ownership, WLCG info, contact lists).

Secondary hierarchy: `resource_groups` → `sites` → `facilities` → `institutions`.

Supporting: `services` (name↔int id), `support_centers`, `vos`, `projects`,
`downtimes`, and `resource_contacts` (resource ↔ user by contact-type + rank).

Contacts are internalized as `users` (provisioned-but-unclaimed until first
login), so an owner can log in and claim a contact responsibility.

### ID compatibility (must reproduce exactly)

- Entity IDs absent from source YAML are generated as
  `1 + (int(md5(name)) % (2**31 - 1))` — facilities, sites, resource groups
  (`GroupID`), resources, services, support centers, VOs.
- Legacy contact IDs are `SHA1(lowercased trimmed email)`; new-style are OSG IDs
  (`OSG…`). Both are preserved.

### Institutions

Fetched live from the external institutions API (OSG IID ↔ ROR), but cached
aggressively in Postgres. Because institution IDs are immutable, an API failure
is a **soft failure**: we serve the cached copy.

### Soft delete

Every domain table has `deleted_at` / `deleted_by`; active-row uniqueness is
enforced with partial unique indexes `... WHERE deleted_at IS NULL`.

## Change-proposal workflow (propose → iterate → approve)

Create/edit/delete never touch live tables directly. Instead:

- **`change_proposals`** — `entity_kind`, `target_id` (null for new
  registrations), `operation` (create/update/delete), `proposed_state JSONB` (the
  mutable head), `base_version` (snapshot of the live row it branched from, for
  conflict detection), `status` (`draft` → `pending` → `approved` / `rejected` /
  `applied` / `withdrawn`), `created_by`, `assigned_reviewer`, timestamps.
- **`change_proposal_revisions`** — append-only
  `(proposal_id, revision_no, proposed_state JSONB, edited_by, note, created_at)`.
  Both the submitter and managers/admins iterate by appending revisions until the
  change is official; approval applies the head revision to the live tables (as a
  soft-delete + insert so entities stay versioned).

Dashboard views derive from this: *my resources*, *my pending registrations*
(`operation=create`), *pending approvals* (proposals I can review).

## Audit log

**`audit_log`** — append-only `(id, actor_user_id, action, entity_kind,
entity_id, proposal_id, detail JSONB, created_at)`. Every state change writes a
row: proposal created/edited/submitted/approved/rejected/applied, role claimed
via invite, identity linked/unlinked, soft-delete, restore. No updates or
deletes are permitted on this table.

## Invites & role claims (from FabAID, extended)

Beyond admin-created account invites, an owner can generate an invite link scoped
to a **responsibility** (e.g. Security Contact on a resource). The invitee opens
the link, onboards via OIDC (creating/linking an identity), and accepts the
responsibility. Invite tokens are single-use, hashed at rest, and expire.

## Restore (administrator only)

- **GitHub round-trip:** import the existing topology YAML tree into Postgres and
  export it back byte-for-byte (modulo whitespace). Proven by a round-trip test.
- Database backups are handled at the infrastructure layer (Postgres's own
  backup mechanism), not by this app.
