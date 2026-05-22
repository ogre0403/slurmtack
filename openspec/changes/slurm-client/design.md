## Context

The `slurm.Client` interface defines 5 methods (SubmitPlaceholderJob, GetNodeState, DrainNode, ResumeNode, CancelJob) that map directly to slurmrestd v0.0.38 operations. The staging environment runs slurmrestd with JWT authentication enabled. The daemon will call slurmrestd over HTTP from the same network (host mode docker).

slurmrestd v0.0.38 uses the path prefix `/slurm/v0.0.38/` and returns JSON responses with a consistent error structure (`errors` array in response body).

## Goals / Non-Goals

**Goals:**

- Provide a production-ready slurmrestd client implementing `slurm.Client`
- Handle JWT auth (token provided at construction, passed as Bearer header)
- Map slurmrestd response structures to the existing Go types (`NodeState`, `PlaceholderJobResult`)
- Provide clear error messages when slurmrestd returns errors
- Support integration testing against real slurmrestd

**Non-Goals:**

- JWT token rotation or auto-refresh (token is long-lived, configured externally)
- slurmrestd version negotiation or multi-version support
- Connection pooling tuning (stdlib http.Client defaults are sufficient)
- Retry logic (handled by the engine/orchestrator layer above)
- Placeholder job script content (that's the placeholder-agent change)

## Decisions

### HTTP Client: stdlib net/http

**Choice**: Use `net/http` with a configured `http.Client` (timeouts)

**Alternatives considered**:
- resty/go-resty: Adds fluent API but pulls in an unnecessary dependency for 5 endpoints
- hashicorp/go-retryablehttp: Retry belongs in the orchestrator, not the HTTP client

**Rationale**: slurmrestd is a simple REST API. Five endpoints don't justify an HTTP client library. Stdlib gives full control over timeouts and keeps dependencies minimal.

### Endpoint Mapping

| Interface Method | HTTP Method | slurmrestd Endpoint | Notes |
|---|---|---|---|
| SubmitPlaceholderJob | POST | `/slurm/v0.0.38/job/submit` | Body: job submission JSON |
| GetNodeState | GET | `/slurm/v0.0.38/node/{name}` | Parse node info from response |
| DrainNode | POST | `/slurm/v0.0.38/node/{name}` | Body: `{"state": "drain", "reason": "..."}` |
| ResumeNode | POST | `/slurm/v0.0.38/node/{name}` | Body: `{"state": "resume"}` |
| CancelJob | DELETE | `/slurm/v0.0.38/job/{id}` | Signal=SIGKILL to force |

### Job Submission Payload

For SubmitPlaceholderJob, the request body follows slurmrestd's job submit schema:

```json
{
  "job": {
    "name": "gpu-switch-<execution_id>",
    "nodes": "1",
    "tasks": "1",
    "exclusive": true,
    "constraint": "<slurm_constraint>",
    "partition": "<partition>",
    "current_working_directory": "/tmp",
    "environment": ["PATH=/usr/bin:/bin"],
    "script": "#!/bin/bash\n# placeholder managed by slurmtack"
  }
}
```

The actual placeholder-agent binary submission will be handled in the placeholder-agent change. For now this submits a minimal script that the engine can cancel later.

### Error Handling

slurmrestd returns errors in the response body as:
```json
{
  "errors": [{"error": "message", "errno": 1234}]
}
```

The client SHALL:
1. Check HTTP status code first (non-2xx = error)
2. Parse `errors` array from response body
3. Return a structured `SlurmAPIError` type containing the HTTP status and error messages
4. Treat connection failures as a separate error type from API-level errors

### Integration Test Strategy

Integration tests require a running slurmrestd. They are gated by:
- Build tag: `//go:build integration`
- Environment variables: `SLURM_API_URL` and `SLURM_JWT_TOKEN` must be set

Tests create real jobs, query real nodes, and clean up after themselves. They run against the staging Slurm cluster, not in CI.

## Risks / Trade-offs

- **slurmrestd API instability** → v0.0.38 is a specific version. Slurm updates may break response schemas. Mitigation: pin to exact version in tests, add response structure validation.
- **JWT expiry during long operations** → If the token expires mid-operation, requests will 401. Mitigation: use long-lived tokens for staging; add token refresh in a future change if needed.
- **Node state parsing complexity** → slurmrestd returns verbose node info. We only need a subset. Mitigation: parse only the fields we use, ignore unknown fields (`json:"-"`).
- **Integration tests affect real cluster** → Tests submit/cancel jobs and drain/resume nodes. Mitigation: use a dedicated test partition/node that won't interfere with production workloads.
