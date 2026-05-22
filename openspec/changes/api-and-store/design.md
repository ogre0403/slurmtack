## Context

The slurmtack daemon has a working orchestration engine with state machine, step handlers, compensation logic, and an in-memory store. Tests drive the engine directly via Go function calls. There is no HTTP interface, no persistent storage, and `cmd/main.go` is a placeholder.

The daemon will run as a single container on a staging host using host networking to reach slurmrestd and OpenStack APIs directly. RabbitMQ runs alongside it in the same docker-compose stack. SQLite is chosen over PostgreSQL for deployment simplicity at this stage.

Operators interact via curl or internal automation tools. The API must be async (switches take minutes), return immediately with an execution ID, and support polling for status.

## Goals / Non-Goals

**Goals:**

- Expose switch lifecycle via REST API that operators can call with curl
- Persist execution and step state across daemon restarts (SQLite)
- Deploy as a single docker-compose stack with RabbitMQ
- Token-based access control (single shared secret for now)
- Graceful shutdown preserving in-flight state
- Health endpoint for monitoring

**Non-Goals:**

- Real Slurm/OpenStack client implementations (separate changes)
- MQ consumer/publisher (separate change: mq-and-orchestrator)
- Orchestrator goroutine that auto-advances state (separate change)
- Multi-tenant auth, RBAC, or OIDC
- TLS termination (handled by network/proxy layer)
- API versioning strategy beyond `/v1/` prefix

## Decisions

### HTTP Framework: Gin

**Choice**: gin-gonic/gin

**Alternatives considered**:
- `net/http` stdlib + chi: Lower dependency count, but more boilerplate for request validation, error responses, and middleware composition
- echo: Similar to Gin but smaller community in Go HPC/infra space

**Rationale**: Gin provides built-in request binding, validation tags, structured error responses, and middleware chaining with minimal setup. The team is familiar with it.

### Store: SQLite via mattn/go-sqlite3

**Choice**: CGO-based SQLite with WAL mode

**Alternatives considered**:
- modernc.org/sqlite (pure Go): No CGO required, but ~20% slower and less battle-tested for concurrent writes
- PostgreSQL: Overkill for single-daemon deployment; adds another container and connection management

**Rationale**: SQLite in WAL mode handles concurrent reads well. Single-writer is acceptable because only one daemon process exists. The CGO requirement is acceptable in a container build (alpine + gcc). File lives on a docker volume, survives restarts.

### Schema Management: Embedded SQL

**Choice**: Embed schema as `CREATE TABLE IF NOT EXISTS` statements run at startup

**Alternatives considered**:
- golang-migrate: Full migration framework; useful later but premature for first schema
- goose: Same reasoning

**Rationale**: There are no existing databases to migrate from. A single `schema.sql` embedded via `//go:embed` keeps the first deployment simple. Switch to a migration tool when the schema needs its first ALTER.

### API Pattern: Async with Polling

**Choice**: POST returns 202 + execution_id; GET polls status

**Alternatives considered**:
- SSE/WebSocket streaming: More complex, no clear consumer need yet
- Webhook callbacks: Requires consumers to run HTTP servers

**Rationale**: Polling is the simplest pattern for curl-based operators and internal scripts. The switch process takes 5-20 minutes; polling every 5-10 seconds is fine.

### Configuration: Environment Variables

**Choice**: Read all config from env vars, no config file

**Alternatives considered**:
- YAML/TOML config file: More structured but adds file management in containers
- Viper: Heavy dependency for simple key-value config

**Rationale**: Env vars are the standard for container deployments. docker-compose `.env` file provides local defaults. Twelve-factor compatible.

### Docker Networking: Host Mode

**Choice**: `network_mode: host` for the daemon container

**Alternatives considered**:
- Bridge network with explicit port mappings: More isolated but daemon needs to reach slurmrestd and OpenStack on arbitrary host-network IPs

**Rationale**: The daemon must reach slurmrestd (typically on the slurm head node's IP) and OpenStack endpoints on the campus/datacenter network. Host mode avoids NAT complexity. RabbitMQ also uses host mode for simplicity (or localhost if co-located).

## Risks / Trade-offs

- **CGO dependency** → Build requires gcc in the Docker build stage. Multi-stage build keeps runtime image small. If CGO becomes a problem later, swap to modernc.org/sqlite with minimal interface changes.
- **Single-writer SQLite** → Only one daemon instance can write. This is fine for single-daemon deployment but blocks horizontal scaling. Acceptable for staging; revisit if production needs HA.
- **Token in env var** → Simple but no rotation mechanism. Acceptable for internal staging. Add token rotation or switch to JWT when moving to production.
- **No orchestrator in this change** → API can create executions and query status, but nothing auto-advances the state machine. Manual state advancement via internal tooling or the next change (mq-and-orchestrator) will add this.
- **Host network mode** → Less isolation. Daemon port (8080) is exposed on the host directly. Acceptable for staging; production should use a reverse proxy.
