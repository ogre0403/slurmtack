## Why

The current SSH reachability path is incomplete for real deployments. The daemon exposes basic SSH settings, but the runtime does not wire an SSH runner into the orchestrator, and the executor has no first-class support for selecting a private key identity, so post-reboot reachability checks cannot reliably authenticate against hosts that require key-based login.

## What Changes

- Add explicit daemon configuration for SSH runner authentication via private key path, alongside the existing SSH user, port, and option settings.
- Wire the SSH executor and runner into daemon startup so reboot and reachability steps use the configured SSH transport instead of failing with `ssh runner not configured`.
- Define validation rules for incomplete SSH configuration so startup and operator feedback are deterministic.
- Cover the completed flow with focused tests and deployment documentation updates.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ssh-reachability`: require the daemon to use configured SSH runner credentials for reboot follow-up polling and host reachability probes.
- `daemon-deployment`: extend the environment-based configuration contract to include the SSH runner settings required for key-based authentication and startup wiring.

## Impact

- Affected code: `internal/config`, `internal/remote`, `internal/orchestrator`, `cmd/main.go`.
- Affected operator surfaces: root `.env.example`, `docker/.env.example`, deployment guidance for mounting or supplying the SSH private key path.
- Validation: new tests for SSH executor argument construction, config validation, and daemon wiring behavior.