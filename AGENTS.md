# AGENTS.md — working in this repo

A Go + Postgres + S3 rewrite of the OSG topology registry. See
[docs/DESIGN.md](docs/DESIGN.md) for the architecture and [README.md](README.md)
for how to run it.

## Layout

- `cmd/server` — entrypoint (also `migrate` subcommand).
- `internal/config` — env config + master-key bootstrap.
- `internal/crypto` — envelope encryption (HKDF from one master key).
- `internal/db` — pgx pool, hand-written `Queries` (no ORM), goose migrations
  (`internal/db/migrations/*.sql`, embedded + auto-run at boot).
- `internal/topology` — YAML data model + reader/writer + importer/exporter
  (the GitHub restore round-trip).
- `internal/xmlapi` — legacy web API output (rgsummary/rgdowntime XML + XSDs).
- `internal/proposalschema` — versioned JSON Schemas + upgraders for change
  proposals (see below).
- `internal/handlers`, `internal/router`, `internal/models`.
- `frontend/` — Next.js + Tailwind SPA (static-exported and embedded in prod).

## Conventions

- Migrations: sequential `NNN_name.sql` with `-- +goose Up` / `-- +goose Down`.
  UUID PKs (`gen_random_uuid()`), `TIMESTAMPTZ`, soft-delete via `deleted_at`
  with partial unique indexes `... WHERE deleted_at IS NULL`.
- Never hard-delete domain rows; soft-delete only.
- Bearer secrets (sessions, invites) are stored as SHA-256 hashes, never plain.
- PII (emails) is envelope-encrypted; the legacy SHA-1 contact id is kept for
  round-trip and lookup.
- DB-backed tests are gated on `TOPOLOGY_TEST_DATABASE_URL` and isolate
  themselves with `internal/testsupport.SetupSchema` (unique per-test schema),
  so `go test ./...` is safe to run concurrently.

## Keeping goose and proposal JSON Schemas in sync

Change proposals store their payload as JSONB (`change_proposals.proposed_state`)
validated against a **versioned JSON Schema** in `internal/proposalschema`. This
JSONB is intentionally **decoupled** from the live table DDL: goose migrates the
live `resources`/`resource_groups`/… tables, while proposals carry their own
`proposed_schema_version` and are brought forward by explicit **upgrader**
functions at apply time. That decoupling is deliberate — an in-flight proposal
must survive a schema change — but it means the two can drift if you are not
careful. Follow this checklist whenever a goose migration changes the shape of a
proposable entity (currently only `resource`):

1. **Decide if `proposed_state` shape changes.** Adding an unrelated column
   (e.g. an index, an audit column) usually does *not* affect the proposal
   payload — no schema bump needed. A change that adds/renames/removes a field
   that appears in `proposed_state` **does**.
2. **Add a new schema file**, do not edit the old one:
   `internal/proposalschema/schema/<kind>_v<N+1>.json`. Keeping old versions lets
   existing proposals validate and upgrade.
3. **Bump `current[<kind>]`** to `N+1` in `proposalschema.go`.
4. **Register an upgrader** `upgraders[<kind>][N] = func(old) (new, error)` that
   transforms a v`N` payload into a v`N+1` payload (rename fields, fill
   defaults, drop removed fields).
5. **Update the apply path** in `internal/handlers/proposals.go` if the new
   fields must be persisted differently (usually `topology.UpsertResource`
   already covers it via the typed model).
6. **Run `go test ./internal/proposalschema/`.** `TestNoUpgraderGaps` fails if
   `current` was bumped without a schema file or an upgrader for every step —
   this is the guard against silent drift. Never make it pass by deleting the
   assertion.

Rule of thumb: **goose migration that touches a proposable entity ⇒ new schema
version + upgrader, in the same PR.** The guard test enforces the mechanics; this
doc explains the intent.

## Build & test

```bash
make build          # server binary (no embedded frontend)
make build-prod     # single binary with embedded Next.js SPA
make test           # go test ./...  (DB tests skip without TOPOLOGY_TEST_DATABASE_URL)
```

Round-trip fidelity is proven by `internal/topology` (import→DB→export equals the
source tree modulo whitespace) and XSD validity by `internal/xmlapi` (xmllint
against the legacy schemas). Run both against real data with
`TOPOLOGY_TEST_REAL_TREE=/path/to/topology/topology`.
