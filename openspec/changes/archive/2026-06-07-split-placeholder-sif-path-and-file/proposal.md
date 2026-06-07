## Why

The current `PLACEHOLDER_SIF_PATH` is one fixed absolute file path, so every placeholder job must reference the same SIF file regardless of which Slurm user submits it. In environments where that path lives under one user's home or another user-private location, other Slurm users can fail to read the image even though the workflow itself is otherwise valid.

## What Changes

- Replace the single `PLACEHOLDER_SIF_PATH` file-path configuration with two inputs: a daemon-configured SIF path pattern rooted under the effective workload user's home directory, plus a default SIF filename used when the request does not override it.
- Extend `POST /v1/switches` for `slurm_to_openstack` with an optional placeholder SIF filename override so callers can select a different SIF filename per execution.
- Define runtime resolution for the placeholder SIF image as: effective workload user -> resolved home directory pattern -> effective SIF filename, where the API-supplied filename overrides the default env filename only for that execution.
- Persist the effective placeholder SIF filename choice needed by the asynchronous placeholder-submission path so retries and restart recovery use the same file selection.
- Update operator-facing deployment docs and examples to show the new env variables, pattern rules, and request-time filename override behavior.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `rest-api`: `POST /v1/switches` accepts an optional placeholder SIF filename override for `slurm_to_openstack`, validates it, and preserves the request choice for the execution lifecycle.
- `slurmrestd-client`: placeholder job submission resolves the SIF path from the effective workload user's home-based path pattern plus the effective filename, rather than using one fixed absolute path.
- `placeholder-agent-packaging`: deployment requirements change from a single shared `PLACEHOLDER_SIF_PATH` to a home-based path pattern and default filename contract that can produce a user-specific resolved SIF path.
- `daemon-deployment`: environment configuration changes from one `PLACEHOLDER_SIF_PATH` value to separate path-pattern and default-filename settings, including documented fallback behavior when the API omits the filename override.

## Impact

- Affected code: `internal/config`, `internal/api`, `internal/service`, `internal/domain`, `internal/store`, `internal/orchestrator`, `internal/slurm`, and related tests.
- Affected APIs: `POST /v1/switches` request schema and validation for `slurm_to_openstack`.
- Affected systems: daemon env configuration, placeholder job submission script generation, execution persistence for async recovery, and operator runbooks/examples for SIF deployment.
