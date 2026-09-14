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

# Stage 3: runtime. Alpine plus a small set of debugging tools.
FROM alpine:3.21
RUN apk add --no-cache \
        ca-certificates \
        bash \
        curl \
        jq \
        bind-tools \
        busybox-extras \
        postgresql16-client \
        procps \
        sudo \
    && adduser -D -u 10001 topology \
    && echo "topology ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/topology \
    && chmod 0440 /etc/sudoers.d/topology
COPY --from=backend /out/topology-server /usr/local/bin/topology-server
# Runs unprivileged by default. To debug as root, either:
#   docker exec -u root -it <container> bash
# or, from a shell inside the container:  sudo -i
USER topology
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/topology-server"]
