# syntax=docker/dockerfile:1

# Stage 1: build the Next.js static export.
FROM node:24-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: build the Go binary with the embedded frontend.
FROM golang:1.26-alpine AS backend
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/out ./internal/frontend/dist
ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 go build -tags embed_frontend \
    -ldflags "-X github.com/bbockelm/topology-v2/internal/version.Version=${VERSION} -X github.com/bbockelm/topology-v2/internal/version.Commit=${COMMIT}" \
    -o /out/topology-server ./cmd/server

# Stage 3: minimal runtime.
FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 topology
COPY --from=backend /out/topology-server /usr/local/bin/topology-server
USER topology
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/topology-server"]
