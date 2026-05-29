## 1. API Access Logging

- [x] 1.1 Extend `internal/api.NewServer` to accept the daemon `*slog.Logger` and add a custom Gin middleware that emits one structured completion log per request.
- [x] 1.2 Ensure the API access log records stable fields such as method, matched route, raw path, status code, latency, and client address while excluding authorization tokens and request bodies.
- [x] 1.3 Update API startup and test helpers to pass the logger through the server constructor without changing existing handler behavior.

## 2. Slurm REST Client Logging

- [x] 2.1 Add logger wiring for `internal/slurm.RestClient` that follows the repo's existing nil-safe `slog` pattern.
- [x] 2.2 Instrument `doRequestWithIdentity` so every slurmrestd call logs method, path, identity type, latency, and response status or transport failure using structured `slog` fields.
- [x] 2.3 Ensure slurmrestd error logs include safe API error summaries without logging JWT token headers or full request bodies.

## 3. Validation

- [x] 3.1 Add focused tests for API access logging coverage, including a successful `/v1` request, an authorization failure, and `/health`.
- [x] 3.2 Add focused tests for slurmrestd client logging coverage, including a successful workload request, a successful admin request, an API rejection, and a transport error.
- [x] 3.3 Run the relevant Go test suites for `internal/api` and `internal/slurm` and confirm the new logging behavior passes without regressions.
