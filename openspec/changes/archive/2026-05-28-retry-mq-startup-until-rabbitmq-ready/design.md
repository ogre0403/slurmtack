## Context

The daemon currently initializes MQ in `cmd/main.go` with a single startup attempt. If `mqConn.Connect(ctx)` fails, the process logs `mq.connect_failed` with `continuing_without_mq=true` and never starts the consumer for that run. The existing reconnect support in `internal/mq.Connection` and `internal/mq.Consumer` only applies after an MQ connection has already been established.

This creates a startup race in `make up`: `slurmtack` and `rabbitmq` start together, but RabbitMQ can take longer to become ready for AMQP than the daemon takes to boot. When that happens, the daemon stays up but permanently misses MQ consumption until an operator restarts it.

## Goals / Non-Goals

**Goals:**
- Ensure that a daemon configured with `AMQP_URL` eventually establishes MQ connectivity when RabbitMQ becomes ready after the daemon starts
- Cover the full MQ startup path with retry behavior: connection establishment, topology declaration, and consumer startup
- Distinguish intentional MQ disablement from temporary MQ unavailability in logs
- Add automated coverage for delayed RabbitMQ readiness during startup

**Non-Goals:**
- Make RabbitMQ mandatory when `AMQP_URL` is unset
- Redesign message schemas, queue topology, or the existing runtime reconnect semantics after steady-state startup
- Depend solely on Docker Compose restart ordering or health checks to solve the problem

## Decisions

### Add a daemon-owned MQ startup supervisor

**Choice:** Replace the one-shot startup branch in `cmd/main.go` with a goroutine that owns initial MQ activation. When `AMQP_URL` is set, the supervisor repeatedly attempts: connect, declare topology, start the consumer. It stops retrying only when startup succeeds or the daemon shuts down.

**Alternatives considered:**
- Fail fast and rely on `restart: unless-stopped`: this only helps Docker Compose deployments, creates restart churn for transient broker delays, and does not fix non-containerized runs.
- Block the entire daemon startup until MQ is ready: this guarantees readiness but keeps the API unavailable during transient broker startup and couples the whole process lifetime to broker timing.

**Rationale:** A startup supervisor fixes the issue in every environment where `AMQP_URL` is configured while preserving API availability. It also matches the existing design direction where MQ outages are handled as recoverable rather than fatal.

### Reuse one retry/backoff policy for initial and steady-state connection attempts

**Choice:** Extend the MQ connection management code so initial startup retries use the same bounded exponential backoff behavior already used for reconnect attempts.

**Alternatives considered:**
- Duplicate retry logic in `cmd/main.go`
- Add fixed sleeps or broker polling only in `docker-compose`

**Rationale:** One retry implementation keeps behavior, logging, and timing consistent. It also avoids a startup-only code path drifting away from the reconnect path over time.

### Treat topology declaration as part of retriable MQ activation

**Choice:** The supervisor considers MQ ready only after connection setup, topology declaration, and consumer launch all succeed. If topology declaration fails after a successful dial, the supervisor closes the partial connection and re-enters the retry loop.

**Alternatives considered:**
- Exit on topology declaration failure
- Retry only the AMQP dial and assume declaration cannot fail independently

**Rationale:** A broker can become reachable before channel-level operations are fully ready. Retrying the full activation sequence prevents another permanent degraded state during startup.

### Log degraded startup explicitly instead of “continuing without MQ”

**Choice:** Keep “MQ disabled” semantics only for the case where `AMQP_URL` is unset. When MQ is configured but unavailable, log retrying/unavailable status and emit a readiness log once the consumer starts.

**Alternatives considered:**
- Keep the existing `continuing_without_mq=true` log message

**Rationale:** Operators need to distinguish between an intentionally disabled integration and a temporary startup race that the daemon is actively recovering from.

## Risks / Trade-offs

- The API and orchestrator can start before MQ is ready, so MQ-dependent executions may still pause briefly at wait states → automatic recovery removes the manual restart requirement and keeps the pause bounded by broker readiness.
- Startup supervision and consumer reconnect both touch MQ lifecycle → the supervisor should own pre-consumer activation, while `consumer.Run` continues to own reconnect after the consumer is active.
- Misconfigured credentials or networking will now retry indefinitely instead of giving up once → exponential backoff limits churn, and repeated logs keep the root cause visible.

## Migration Plan

- Deploy the updated daemon with no configuration changes.
- On rollout, instances with `AMQP_URL` configured will retry until RabbitMQ becomes available instead of staying disconnected for the rest of the process lifetime.
- Rollback is a normal binary/container rollback and restores the current manual-restart behavior.

## Open Questions

- None for this change. Surfacing MQ degraded state through an explicit health/readiness endpoint can be evaluated separately if operators need it.