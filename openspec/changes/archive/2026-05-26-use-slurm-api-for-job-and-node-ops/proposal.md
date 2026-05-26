## Why

The current Slurm integration only partially uses slurmrestd: placeholder job submission and node lookup go through HTTP, but node drain and resume still behave as unsupported in code. The target environment already exposes working Slurm REST endpoints for job submission and node management on slurmrestd v0.0.40, so the implementation and specs need to match the deployed API contract instead of relying on outdated assumptions.

## What Changes

- Update the Slurm client contract and implementation assumptions to use the deployed slurmrestd API for placeholder job submission, node state lookup, node drain, and node resume
- Align request authentication and API version expectations with the environment's Slurm API usage, including distinct operator credentials where node management requires elevated privileges
- Update placeholder-agent polling behavior so it uses the same Slurm API contract as the daemon when checking node drain state
- Extend configuration and operator documentation for the Slurm API identities and tokens needed by job-scoped and admin-scoped operations
- Add or revise focused tests that validate the expected request paths, headers, and payload shapes against the supported Slurm API behavior

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `slurmrestd-client`: change job and node operation requirements to match the deployed slurmrestd API version, request headers, and supported node drain/resume behavior
- `daemon-deployment`: change daemon configuration requirements to cover the Slurm API credentials needed for regular job actions and elevated node management actions
- `placeholder-agent-lifecycle`: change the Slurm polling contract to use the same deployed Slurm API path and authentication model used by the daemon

## Impact

- **Affected code**: `internal/slurm`, `internal/config`, `cmd/main.go`, `cmd/placeholder-agent`, and related tests
- **Configuration**: existing Slurm API settings will be revised and may gain separate user/token inputs for job and admin operations
- **External systems**: depends on the deployed slurmrestd v0.0.40 endpoints remaining available for `/job/submit`, `/job/{id}`, `/jobs/state`, and `/node/{name}` operations
- **Operations**: staging and production runbooks will need to use the documented Slurm API token generation flow for both normal job access and node administration