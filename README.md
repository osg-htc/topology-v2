# OSG Topology Webapp

A Go + Postgres + S3 rewrite of the OSG topology registry, replacing the
hand-edited YAML + GitHub-PR workflow with a resource-centric webapp: federated
login and identity linking, a propose-then-approve change workflow, encrypted
contact PII, and backup/restore that round-trips the existing topology GitHub
repo.

See [docs/DESIGN.md](docs/DESIGN.md) for the full architecture.

## Quick start (dev)

The devcontainer brings up Postgres and MinIO automatically. Outside the
devcontainer:

```bash
# 1. Start Postgres (and optionally MinIO) — e.g. via docker:
docker run -d --name topology-pg -p 5432:5432 \
  -e POSTGRES_USER=topology -e POSTGRES_PASSWORD=topology -e POSTGRES_DB=topology \
  postgres:16-alpine

# 2. Configure and run the backend (migrations run at boot):
export TOPOLOGY_DATABASE_URL="postgres://topology:topology@localhost:5432/topology?sslmode=disable"
make dev            # or: make build && ./bin/topology-server

# 3. Run the frontend dev server (proxies /api to :8080):
cd frontend && npm install && npm run dev
```

Backend: http://localhost:8080 · Frontend: http://localhost:3000

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
