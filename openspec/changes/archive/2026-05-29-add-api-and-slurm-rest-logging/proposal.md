## Why

The daemon currently serves HTTP requests and calls slurmrestd without emitting structured request logs for either path, so operators cannot tell which API call arrived, which outbound Slurm call was attempted, or how those calls completed from the live process logs. This needs to change now because the codebase already standardizes on `log/slog` for daemon tracing, but the REST boundaries remain a blind spot during debugging.

## What Changes

- Add structured access logs for inbound HTTP requests handled by the daemon's REST API, including request method, route, response status, latency, and client address while excluding bearer tokens and request bodies.
- Add structured logs for outbound slurmrestd calls from `internal/slurm.RestClient`, covering request method, path, target identity type, response status, latency, and API error summaries without logging tokens.
- Align the new API and slurmrestd log fields with the existing `log/slog` usage so REST-boundary logs can be filtered alongside current daemon trace events.
- Add focused tests for API middleware and slurmrestd client behavior to verify the required log events and redaction rules.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `rest-api`: require structured access logging for inbound daemon HTTP requests.
- `slurmrestd-client`: require structured logging for outbound slurmrestd requests and responses.
- `gpu-node-switch-observability`: extend daemon observability requirements so REST-boundary logs use the same structured logging approach and safe redaction rules as the rest of the daemon.

## Impact

Affected code is expected in `internal/api`, `internal/slurm`, `cmd/main.go`, and related tests in `internal/api/*test.go` and `internal/slurm/*test.go`. The change affects operational logging only, with no database schema change and no external API contract change beyond the new observability guarantees.
