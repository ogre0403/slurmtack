## Why

The current `slurm_to_openstack` flow assumes one global workload Slurm identity and a minimal placeholder job payload. That breaks down in environments where placeholder jobs must charge a specific Slurm account, write logs under the submitting user's home directory, or use per-request Slurm credentials instead of a daemon-wide token.

## What Changes

- Extend `POST /v1/switches` for `slurm_to_openstack` so callers can supply a requested Slurm account plus request-scoped Slurm workload credentials that override the daemon defaults for that execution.
- Define effective workload identity resolution for `slurm_to_openstack`: use request-supplied Slurm user and token when both are provided; otherwise fall back to `SLURM_API_USER` and `SLURM_JWT_TOKEN`; reject the request when neither source provides a complete workload identity.
- Update placeholder job submission so the Slurm `job` object can include an `account` value and uses the effective Slurm user's home directory for `current_working_directory`, `standard_output`, and `standard_error` instead of `/tmp`.
- Carry the execution-scoped Slurm submission settings through the asynchronous placeholder allocation lifecycle, expose the non-secret `slurm_account` in execution detail APIs, and keep credentials out of read APIs and logs.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `rest-api`: `POST /v1/switches` accepts `slurm_to_openstack` account and workload-credential overrides, with request-time validation when no effective Slurm identity is available, and `GET /v1/switches/:id` returns the requested Slurm account in execution detail metadata.
- `gpu-node-switch-allocation`: placeholder submission includes the requested Slurm account and continues to use the execution's effective workload identity while the switch remains allocation-bound.
- `slurmrestd-client`: placeholder job submissions include account-aware payload fields, user-home log paths, and the effective execution-scoped Slurm identity.
- `daemon-deployment`: environment Slurm workload credentials become optional defaults for request fallback instead of an unconditional startup requirement for `slurm_to_openstack`.

## Impact

- Affected code: `internal/api`, `internal/service`, `internal/domain`, `internal/store`, `internal/orchestrator`, `internal/slurm`, config loading, and related tests.
- Affected APIs: `POST /v1/switches` request schema and validation rules for `slurm_to_openstack`, plus `GET /v1/switches/:id` execution detail metadata.
- Affected systems: daemon configuration defaults, SQLite execution persistence, placeholder job submission payloads, and operator runbooks/examples for Slurm API usage.
