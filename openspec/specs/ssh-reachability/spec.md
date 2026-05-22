## ADDED Requirements

### Requirement: SSH poll after reboot

The orchestrator SHALL poll a rebooted host via SSH at a configurable interval (default 10s) until the host responds successfully or a timeout is reached (default 10 minutes).

#### Scenario: Host comes back within timeout

- **WHEN** an execution is in state `rebooting` and SSH connection succeeds within the timeout period
- **THEN** orchestrator transitions to `host_reachable`

#### Scenario: Host does not come back

- **WHEN** an execution is in state `rebooting` and SSH poll times out (no successful connection within configured timeout)
- **THEN** orchestrator fails the execution with failure class `unknown_after_reboot` and transitions to `failed_manual_recovery`

### Requirement: SSH probe command

The SSH poll SHALL execute a minimal command (`hostname`) to verify the host is responsive. The probe MUST use the existing `remote.Runner` interface with a short per-attempt timeout (5 seconds).

#### Scenario: Probe succeeds

- **WHEN** SSH connection is established and `hostname` returns exit code 0
- **THEN** the host is considered reachable

#### Scenario: Probe connection refused

- **WHEN** SSH connection is refused (host not yet booted)
- **THEN** poll waits for the next interval and retries

#### Scenario: Probe timeout

- **WHEN** SSH connection hangs (host partially up)
- **THEN** the per-attempt timeout fires, poll waits for the next interval and retries

### Requirement: Configurable poll parameters

The SSH poll interval and overall timeout MUST be configurable via environment variables `SSH_POLL_INTERVAL` (default "10s") and `SSH_POLL_TIMEOUT` (default "10m").

#### Scenario: Custom interval

- **WHEN** `SSH_POLL_INTERVAL=5s` is set
- **THEN** the orchestrator polls every 5 seconds instead of the default 10

#### Scenario: Custom timeout

- **WHEN** `SSH_POLL_TIMEOUT=15m` is set
- **THEN** the orchestrator allows up to 15 minutes before declaring failure
