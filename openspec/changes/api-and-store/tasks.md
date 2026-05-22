## 1. Dependencies and Configuration

- [ ] 1.1 Add go dependencies: gin-gonic/gin, mattn/go-sqlite3
- [ ] 1.2 Create `internal/config/config.go` with env-based config struct (API_TOKEN, LISTEN_ADDR, DB_PATH, plus optional SLURM_API_URL, SLURM_JWT_TOKEN, OS_AUTH_URL, OS_PROJECT_NAME, OS_USERNAME, OS_PASSWORD, AMQP_URL)
- [ ] 1.3 Add config validation (exit with clear error on missing required vars)

## 2. SQLite Store

- [ ] 2.1 Create `internal/store/schema.sql` with tables for executions, steps, and leases (embed via go:embed)
- [ ] 2.2 Implement `internal/store/sqlite.go` satisfying the Store interface with WAL mode and busy_timeout=5000
- [ ] 2.3 Implement schema initialization (CREATE TABLE IF NOT EXISTS on Open)
- [ ] 2.4 Implement CreateExecution, GetExecution, ListExecutions, UpdateExecution
- [ ] 2.5 Implement AdvanceState with optimistic concurrency (WHERE state_version = expected)
- [ ] 2.6 Implement AcquireLease, ReleaseLease, GetLease with exclusivity enforcement
- [ ] 2.7 Implement CreateStep, UpdateStep, ListSteps (ordered by sequence)
- [ ] 2.8 Write `internal/store/sqlite_test.go` covering all Store interface methods

## 3. REST API

- [ ] 3.1 Create `internal/api/dto.go` with request/response structs (SwitchRequest, SwitchResponse, ExecutionStatus, ErrorResponse)
- [ ] 3.2 Create `internal/api/middleware_auth.go` with bearer token validation middleware
- [ ] 3.3 Create `internal/api/handler_switch.go` with POST /v1/switches (202 async pattern)
- [ ] 3.4 Create `internal/api/handler_switch.go` with GET /v1/switches/:id (execution detail)
- [ ] 3.5 Create `internal/api/handler_switch.go` with GET /v1/switches (list with ?node= and ?status= filters)
- [ ] 3.6 Create `internal/api/handler_switch.go` with POST /v1/switches/:id/cancel (501 stub)
- [ ] 3.7 Create `internal/api/handler_health.go` with GET /health (check SQLite reachable)
- [ ] 3.8 Create `internal/api/server.go` with Gin engine setup, route registration, and graceful shutdown
- [ ] 3.9 Write API integration tests (start server, hit endpoints, verify responses)

## 4. Entrypoint

- [ ] 4.1 Rewrite `cmd/main.go` to wire config → SQLite store → SwitchService → Gin server
- [ ] 4.2 Add signal handling (SIGTERM/SIGINT) with graceful shutdown (10s timeout)
- [ ] 4.3 Verify `go build ./cmd/...` succeeds

## 5. Docker and Deployment

- [ ] 5.1 Create `Dockerfile` with multi-stage build (golang+gcc build stage, alpine runtime stage, non-root user)
- [ ] 5.2 Create `docker/docker-compose.yaml` with slurmtack and rabbitmq services, host network mode, volume for SQLite
- [ ] 5.3 Create `docker/.env.example` with all config variables documented
- [ ] 5.4 Verify `docker-compose build` succeeds
- [ ] 5.5 Verify `docker-compose up` starts both services and API responds on :8080/health
