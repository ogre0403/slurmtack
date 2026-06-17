## Why

`SLURM_ADMIN_JWT_TOKEN` is currently read once from environment and then reused for the lifetime of the daemon. Because Slurm admin JWTs expire, long-running deployments eventually lose the ability to drain, resume, or perform other admin-authenticated Slurm operations until the daemon is restarted with a fresh token.

## What Changes

- Add a deployment path that can mint short-lived Slurm admin JWTs over SSH by connecting to a configured login node and running `scontrol token`.
- Keep `SLURM_ADMIN_JWT_TOKEN` for backward compatibility, but make SSH-based renewal the primary admin-token source when `SSH_LOGIN_NODE` is configured.
- Refactor admin-authenticated slurmrestd operations to resolve admin credentials at request time, cache the current token in memory, and retry once after refreshing the token on authentication failure.
- Document the new environment variables, precedence rules, and operational expectations for SSH access and token lifespan.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `daemon-deployment`: change Slurm admin token configuration to support SSH-based runtime renewal while preserving static-token compatibility.
- `slurmrestd-client`: add runtime admin-token acquisition, cache invalidation, one-time retry behavior, and datastore-backed renewal auditing for admin-authenticated Slurm API operations.
- `sqlite-store`: persist an audit trail for SSH-minted admin-token renewal events without storing token material.

## Impact

- Affected code: `internal/config`, `cmd/main.go`, `internal/slurm`, and deployment-facing docs/env examples.
- Affected systems: Slurm admin API access now depends on optional SSH access to a login node when renewal is enabled.
- No API contract change for existing switch endpoints; the change is internal to deployment configuration and Slurm integration behavior.
