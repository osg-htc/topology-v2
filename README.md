# OSG Topology Webapp

A Go + Postgres + S3 rewrite of the OSG topology registry, replacing the
hand-edited YAML + GitHub-PR workflow with a resource-centric webapp: federated
login and identity linking, a propose-then-approve change workflow, encrypted
contact PII, and backup/restore that round-trips the existing topology GitHub
repo.

See [docs/DESIGN.md](docs/DESIGN.md) for the full architecture.

## Quickest start — one command (Docker)

Bring up Postgres + MinIO + the app (frontend embedded) with a single command:

```bash
make up            # == docker compose up --build
```

Then open **http://localhost:8080**. It starts in development mode, so use the
**Dev login** box on the login page (pick a role, e.g. `administrator`) — no
OIDC setup needed.

To load real data once you're in: go to **Admin → Backup & restore → Import from
GitHub** (pulls `opensciencegrid/topology`). Or, from a checkout of the topology
repo, run the importer directly (see below).

Other endpoints while it's up: MinIO console at http://localhost:9101
(minioadmin / minioadmin). Stop with `make down` (keeps data) or `make
down-clean` (wipes volumes).

Host ports are deliberately non-default to avoid clashing with other local
stacks: app **:8080**, Postgres **:55432**, MinIO API **:9100**, console
**:9101** (the app reaches Postgres/MinIO over the internal Docker network, so
those mappings are only for optional external access). If :8080 is taken, add a
`docker-compose.override.yml` remapping the `app` port.

### Play with the API directly

```bash
# Legacy-compatible read API (works once data is loaded):
curl http://localhost:8080/rgsummary/xml
curl http://localhost:8080/rgdowntime/xml
curl http://localhost:8080/miscfacility/json
curl http://localhost:8080/schema/rgsummary.xsd

# Authenticated flows use a session cookie from dev-login:
curl -c cookies.txt -X POST http://localhost:8080/api/v1/auth/dev-login \
  -H 'Content-Type: application/json' -d '{"email":"you@osg.org","role":"administrator"}'
curl -b cookies.txt http://localhost:8080/api/v1/dashboard
```

## Local dev (hot reload, without Docker for the app)

Bring up just the datastores, then run backend + frontend natively for fast
iteration:

```bash
# datastores (or use the compose services above)
docker run -d --name topology-pg -p 5432:5432 \
  -e POSTGRES_USER=topology -e POSTGRES_PASSWORD=topology -e POSTGRES_DB=topology \
  postgres:16-alpine

export TOPOLOGY_DATABASE_URL="postgres://topology:topology@localhost:5432/topology?sslmode=disable"

# optional: seed from a local checkout of the topology repo
go run ./cmd/server import-tree /path/to/topology/topology

make dev                              # backend with air hot-reload (:8080)
cd frontend && npm install && npm run dev   # frontend dev server (:3000)
```

Backend: http://localhost:8080 · Frontend dev server: http://localhost:3000
(proxies `/api` to the backend). A VS Code **devcontainer**
(`.devcontainer/`) is also provided.

## End-to-end (Playwright) tests

```bash
cd frontend
npm run test:e2e        # spins up the stack expectation; see e2e/README
```

## Configuration

All config comes from `TOPOLOGY_*` environment variables (see
`internal/config/config.go`). Key ones:

| Var | Purpose |
| --- | --- |
| `TOPOLOGY_DATABASE_URL` | Postgres connection string (required) |
| `TOPOLOGY_INSTANCE_KEY` | 32-byte hex master key (auto-generated if unset) |
| `TOPOLOGY_S3_*` | S3/MinIO endpoint, bucket, credentials (backups) |
| `TOPOLOGY_OIDC_*` | OIDC issuer / client id / secret (CILogon by default) |
| `TOPOLOGY_INSTITUTIONS_API` | External institutions registry (cached in DB) |
| `TOPOLOGY_GITHUB_*` | topology repo + token for backup/restore |

## Build

```bash
make build        # server binary (frontend served by Next.js dev server)
make build-prod   # single binary with the frontend embedded via go:embed
make docker       # production container image
make test         # Go tests (includes the YAML round-trip test)
```
