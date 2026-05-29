## Context

The daemon already uses `log/slog` for process lifecycle logs, switch request acceptance, orchestrator trace events, MQ lifecycle events, and SSH dispatch tracing. Two REST boundaries still do not participate in that structured logging model:

- `internal/api.NewServer` builds a Gin engine with recovery only, so inbound HTTP requests are not logged at all.
- `internal/slurm.RestClient` issues outbound HTTP calls directly through `http.Client.Do` and returns parsed errors, but it does not emit request or response logs.

This leaves operators without a live record of which REST API endpoint was called, which slurmrestd endpoint was contacted, how long those operations took, or whether failures came from transport errors or API rejections. The change should fill that observability gap while matching the current logging style and avoiding token leakage.

## Goals / Non-Goals

**Goals:**

- Add structured inbound API access logs using `log/slog` rather than Gin's default text logger.
- Add structured outbound slurmrestd client logs around every HTTP request and response path.
- Keep sensitive values out of logs, especially bearer tokens, Slurm JWT tokens, and full request bodies.
- Reuse existing constructor and logger patterns so the change remains small and consistent with the rest of the daemon.
- Add focused tests that assert event names, core fields, and redaction behavior.

**Non-Goals:**

- Changing REST API request or response payloads.
- Logging full request or response bodies for either inbound API or outbound slurmrestd traffic.
- Introducing a new logging library or HTTP framework.
- Reworking the broader trace helper model outside the files needed for API and slurmrestd logging.

## Decisions

### Decision: Add a custom Gin middleware for API access logging

The API server will keep using `gin.New()` with `gin.Recovery()`, but it will insert a dedicated middleware that logs one structured entry per completed request. The middleware will derive its logger from the server's injected base logger and record stable fields such as `component=api`, `event=api.request`, `method`, `route`, `path`, `status_code`, `latency`, and `client_ip`.

Rationale:

- The repo already standardized on `slog`, so using Gin's built-in logger would reintroduce a different text logging style.
- Middleware is the smallest change that covers all handlers, including auth failures and panic recovery outcomes.
- Logging once on completion keeps volume low while still capturing status and latency.

Alternatives considered:

- Use `gin.Logger()`: rejected because its format and field model would not match the daemon's `slog` output.
- Log independently inside each handler: rejected because it would miss auth rejections, health checks, and future routes unless every handler remembered to log.

### Decision: Thread the base logger into `api.NewServer`

`api.NewServer` will take a `*slog.Logger` so the server can build middleware with the same base logger that `cmd/main.go` already creates. The server will default through the existing trace helper pattern when a nil logger is passed.

Rationale:

- The base logger already exists in `cmd/main.go`; reusing it preserves handler configuration and output formatting.
- Constructor injection matches the current service, MQ, orchestrator, and SSH logging approach.

Alternatives considered:

- Use `slog.Default()` directly inside `internal/api`: rejected because tests and daemon startup already control logger wiring explicitly, and constructor injection is clearer.

### Decision: Log slurmrestd calls inside `RestClient.doRequestWithIdentity`

The slurmrestd client will emit structured logs at the common request boundary in `doRequestWithIdentity`, because every public RestClient method already passes through that function. The log model will distinguish three outcomes:

- request completed with HTTP response
- request failed at transport level before a response arrived
- request completed with an API error response

Representative fields:

- `component=slurmrestd_client`
- `event=slurmrestd.request`
- `method`
- `path`
- `status_code` when available
- `latency`
- `identity` with values such as `workload` or `admin`
- `error` or summarized API error messages when present

Rationale:

- Instrumenting the shared request function guarantees consistent coverage across submit, node lookup, drain, resume, and cancel flows.
- The caller already knows whether admin or workload credentials were used, so the log can expose identity type without exposing credentials.
- Logging after the request completes captures both latency and final status in one entry.

Alternatives considered:

- Log in each public RestClient method separately: rejected because field names and error handling would drift.
- Wrap the HTTP transport with a custom `RoundTripper`: rejected because the client also needs semantic fields like `identity`, and the shared function is simpler.

### Decision: Prefer route templates and redacted metadata over raw payload details

Inbound API logs should prefer the matched Gin route template when available, for example `/v1/switches/:id`, and may include the raw path separately for debugging. Outbound slurmrestd logs will record only method, path, status, latency, and error summaries. Neither side will log Authorization headers, Slurm token headers, or request bodies.

Rationale:

- Route templates produce stable aggregation keys while raw paths preserve enough operational context.
- This change is about traceability, not full audit-body capture.
- Existing observability work already treats sensitive material conservatively.

Alternatives considered:

- Log full headers and bodies for completeness: rejected because it would leak secrets and create noisy, hard-to-search logs.

## Risks / Trade-offs

- [Request logging could increase log volume on busy API polling paths] → Keep one completion log per request and avoid body logging.
- [Field drift between API and slurmrestd logs could make filtering harder] → Define stable event names and component fields in the new middleware and client helper tests.
- [Transport errors may be double-reported if callers also log returned errors] → Keep the boundary log concise and let higher-level workflow logs continue to explain execution impact.
- [Constructor signature changes can ripple through tests] → Limit the API constructor change to `NewServer` and use nil-safe logger defaults where possible.

## Migration Plan

1. Extend `api.NewServer` to accept the daemon logger and add a new access-log middleware.
2. Add a logger option or constructor wiring for `slurm.RestClient` and instrument `doRequestWithIdentity` with structured completion logs.
3. Update startup code and affected tests to pass loggers explicitly where needed.
4. Add unit tests that capture logs for representative API requests, auth failures, successful slurmrestd calls, API errors, and transport failures.
5. Roll back by removing the middleware and RestClient instrumentation; there is no persistence or protocol migration.

## Open Questions

- Should health checks log at the same level as authenticated `/v1` requests, or should they use a lower level if they become noisy in production?
- Should slurmrestd API error logs include the full `Messages` slice or a joined summary string to keep downstream log consumption simpler?
