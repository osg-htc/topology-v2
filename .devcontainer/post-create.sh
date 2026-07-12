#!/usr/bin/env bash
set -euo pipefail

echo "==> Installing air (Go hot-reload)"
go install github.com/air-verse/air@latest || true

echo "==> Downloading Go modules"
go mod download

echo "==> Installing frontend dependencies"
if [ -f frontend/package.json ]; then
  (cd frontend && npm install)
fi

echo "==> Running database migrations"
go run ./cmd/server migrate up || echo "migrations will run at server boot"

echo "Dev container ready. Backend: 'make dev'. Frontend: 'cd frontend && npm run dev'."
