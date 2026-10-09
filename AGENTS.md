# AGENTS.md — working in this repo

A Go + Postgres + S3 rewrite of the OSG topology registry. See
[docs/DESIGN.md](docs/DESIGN.md) for the architecture and [README.md](README.md)
for how to run it.

## Layout

- `cmd/server` — entrypoint (also `migrate` subcommand).
- `internal/config` — env config + master-key bootstrap.
- `internal/crypto` — envelope encryption (HKDF from one master key).
- `internal/conv` — shared conversions between loosely-typed YAML/JSON values and typed ones (JSON decode/encode for storage, bool defaults). Look here before writing one.
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

## Look for an existing helper first

Before writing any small utility — decoding stored JSON/YAML, "map to JSON or
nil", "bool with a default", "explicit ID else the name hash", building a
list-of-names query — **search the repo for one** (`grep -rn "func .*Keyword"
internal`). If it exists, use it; if a near-copy exists, merge the two rather
than adding a third.

Known homes:

- `internal/conv` — `MapFromJSON` / `AnyFromJSON` / `DecodeJSONObject`,
  `JSONOrNil` / `JSONAnyOrNil`, `BoolOr` (for `*bool`), `MapBool` (for a decoded
  map). Add new conversions of this kind here, not as a private copy.
- `topology.ResolveID` / `IDOrGen` / `ProjectIDOrGen` — the one place the rule
  "an explicit id wins, otherwise `GenID(name)`" lives.
- `db.Queries.childNames` — the "list live names under a parent" query helper
  behind the delete guards.

Why this is a rule: these used to exist as several private copies that drifted
apart. One decoded numbers as `float64`, another as `int`, one treated a nil map
as "no value" and another stored the JSON text `null`. The `float64` copy wrote
`KSI2KMax: 15600000` back out of a backup as `1.56e+07`, and a YAML 1.1 reader
(v1's PyYAML) reads `2e+06` as a *string*. A fix applied to one copy never
reached the others.

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
