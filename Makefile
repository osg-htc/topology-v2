.PHONY: help dev build build-prod migrate migrate-down migrate-status test docker frontend-build clean

# Version metadata injected at build time.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -X github.com/bbockelm/topology-v2/internal/version.Version=$(VERSION) \
           -X github.com/bbockelm/topology-v2/internal/version.Commit=$(COMMIT)

DATABASE_URL ?= postgres://topology:topology@localhost:5432/topology?sslmode=disable

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

dev: ## Run backend (air hot-reload) — run the Next.js dev server separately
	air

build: ## Build the server binary (no embedded frontend)
	go build -ldflags "$(LDFLAGS)" -o bin/topology-server ./cmd/server

build-prod: frontend-build ## Build single binary with embedded frontend
	rm -rf internal/frontend/dist && cp -r frontend/out internal/frontend/dist
	go build -tags embed_frontend -ldflags "$(LDFLAGS)" -o bin/topology-server ./cmd/server

frontend-build: ## Export the Next.js SPA to frontend/out
	cd frontend && npm ci && npm run build

migrate: ## Apply pending migrations
	go run ./cmd/server migrate up

migrate-status: ## Show migration status
	go run ./cmd/server migrate status

test: ## Run Go tests
	go test ./...

docker: ## Build the production Docker image
	docker build -t topology-webapp:$(VERSION) .

clean: ## Remove build artifacts
	rm -rf bin internal/frontend/dist frontend/out
