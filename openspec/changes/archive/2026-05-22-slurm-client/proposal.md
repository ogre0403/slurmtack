## Why

The `slurm.Client` interface exists but has no real implementation — only fakes used in tests. The daemon needs to interact with a real Slurm cluster via slurmrestd v0.0.38 (JWT-authenticated) to submit placeholder jobs, drain/resume nodes, query node state, and cancel jobs. Without this, the orchestration engine cannot drive actual GPU node switches.

## What Changes

- Implement `slurm.Client` interface backed by slurmrestd v0.0.38 HTTP API with JWT authentication
- Add JWT token management (configured via env, passed as Bearer header)
- Map all 5 interface methods to their corresponding slurmrestd endpoints
- Add integration tests that run against a real slurmrestd instance (gated by build tag or env var)
- Handle slurmrestd-specific error responses and map to meaningful Go errors

## Capabilities

### New Capabilities

- `slurmrestd-client`: HTTP client implementation targeting slurmrestd v0.0.38, covering job submission, node state queries, node drain/resume, and job cancellation with JWT auth

### Modified Capabilities

(none — the `slurm.Client` interface contract remains unchanged)

## Impact

- **New files**: `internal/slurm/restclient.go`, `internal/slurm/restclient_test.go`
- **New dependencies**: `net/http` (stdlib only, no external HTTP client library needed)
- **External systems**: Requires access to slurmrestd v0.0.38 endpoint for integration tests
- **Configuration**: Adds `SLURM_API_URL` and `SLURM_JWT_TOKEN` env vars (already stubbed in api-and-store change)
