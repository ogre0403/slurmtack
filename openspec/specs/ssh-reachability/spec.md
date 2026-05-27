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

### Requirement: Configured SSH runner transport

The daemon SHALL use a configured SSH runner transport for reboot and reachability operations. That transport MUST apply the configured `SSH_USER`, `SSH_PORT`, `SSH_OPTIONS`, and `SSH_PRIVATE_KEY_PATH` values to both the `reboot` command and the post-reboot `hostname` probe. The transport MUST render the remote command payload from `Command` and `Args` only; `execution_id`, `step_name`, and other workflow metadata MAY be used for local correlation and logging but MUST NOT be appended to the remote command line.

#### Scenario: Reboot command uses configured identity

- **WHEN** an execution reaches the reboot step and the daemon has `SSH_USER=slurm`, `SSH_PORT=2222`, `SSH_PRIVATE_KEY_PATH=/run/secrets/node-key`, and `SSH_OPTIONS=StrictHostKeyChecking=accept-new ConnectTimeout=5`
- **THEN** the daemon issues the `reboot` command through the SSH runner using target `slurm@<host>`, port `2222`, the configured private key file, the configured SSH options, and a rendered remote command of exactly `reboot`

#### Scenario: Reachability probe uses the same transport

- **WHEN** an execution is in state `rebooting` and the orchestrator polls host reachability
- **THEN** each `hostname` probe uses the same configured SSH user, port, private key file, and SSH options as the reboot command, and the rendered remote command is exactly `hostname`

#### Scenario: Workflow metadata does not mutate the remote payload

- **WHEN** the daemon dispatches a reboot or reachability probe with `execution_id` and `step_name` metadata
- **THEN** that metadata is retained only for local correlation and does not appear in the remote command sent over SSH
