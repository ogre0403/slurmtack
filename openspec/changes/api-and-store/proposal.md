## Why

The core orchestration engine exists but has no external interface. Operators cannot trigger switches or query status without calling Go functions directly. The system also loses all state on restart because only an in-memory store exists. Before integrating real Slurm and OpenStack clients, we need a deployable daemon with a stable API surface and durable storage.

## What Changes

- Add a RESTful HTTP API (Gin) exposing switch lifecycle operations (request, status, list, cancel stub)
- Add token-based authentication middleware for API access control
- Implement a SQLite-backed store satisfying the existing `Store` interface
- Add environment-based configuration loading for all runtime parameters
- Rewrite `cmd/main.go` to wire config, store, service, and HTTP server with graceful shutdown
- Provide a Dockerfile (multi-stage build) and docker-compose.yaml (host network, RabbitMQ, SQLite volume)
- Add a `/health` liveness endpoint

## Capabilities

### New Capabilities

- `rest-api`: HTTP API surface for switch lifecycle operations (request, status, list, cancel) with token auth and async response patterns
- `sqlite-store`: Durable SQLite implementation of the Store interface with schema migrations
- `daemon-deployment`: Dockerfile, docker-compose, and configuration for deploying the daemon alongside RabbitMQ on a staging host

### Modified Capabilities

- `gpu-node-switch-orchestration`: Adding the wiring layer that connects API requests to the existing engine (no requirement-level changes, just integration plumbing)

## Impact

- **New packages**: `internal/api/`, `internal/config/`
- **Modified packages**: `internal/store/` (add sqlite.go), `cmd/` (rewrite main.go)
- **New dependencies**: `gin-gonic/gin`, `mattn/go-sqlite3` (CGO), `golang-migrate` or embedded schema
- **Deployment**: New Dockerfile and docker-compose.yaml replace the placeholder docker/ directory
- **Breaking**: None (no existing consumers)
