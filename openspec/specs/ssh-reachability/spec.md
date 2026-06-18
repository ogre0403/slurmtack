## Purpose

Define the SSH-based reboot reachability behavior, probe semantics, and transport configuration used by the orchestrator.
## Requirements
### Requirement: SSH poll after reboot

The orchestrator SHALL poll a rebooted host via SSH at a configurable interval (default 10s) until the reboot is confirmed or a timeout is reached (default 10 minutes). Reboot confirmation MUST require evidence that the host became unreachable after the reboot command was issued and later responded successfully to SSH.

#### Scenario: Host becomes unreachable and returns within timeout

- **WHEN** an execution is in state `rebooting`, at least one SSH probe fails after reboot is initiated, and a later SSH probe succeeds within the timeout period
- **THEN** the orchestrator transitions to `host_reachable`

#### Scenario: Immediate probe succeeds before reboot begins

- **WHEN** an execution is in state `rebooting` and an SSH probe succeeds before any failed probe has been observed
- **THEN** the orchestrator keeps the execution in `rebooting` and continues polling for evidence that the reboot has started

#### Scenario: Host never becomes unreachable

- **WHEN** an execution is in state `rebooting` and SSH probes keep succeeding until the SSH poll timeout is reached without any failed probe
- **THEN** the orchestrator fails the execution with failure class `unknown_after_reboot` and transitions to `failed_manual_recovery`

#### Scenario: Host becomes unreachable and does not come back

- **WHEN** an execution is in state `rebooting`, at least one SSH probe fails after reboot is initiated, and no later SSH probe succeeds before the timeout period ends
- **THEN** the orchestrator fails the execution with failure class `unknown_after_reboot` and transitions to `failed_manual_recovery`

### Requirement: SSH probe command

The SSH poll SHALL execute a command that verifies the host has finished booting, not merely that sshd accepts connections. The probe MUST confirm the node is out of the `pam_nologin` window (e.g. `/run/nologin` is absent) before reachability is satisfied. The probe MUST use the existing `remote.Runner` interface with a short per-attempt timeout (5 seconds). A successful probe SHALL satisfy the reboot wait only after the reboot has already been observed through a prior failed probe.

A probe that establishes an SSH connection but is rejected because the system is still booting (for example `pam_nologin` reporting "System is booting up. Unprivileged users are not permitted to log in yet") MUST be treated as reboot progress that is not yet complete: it counts as evidence the reboot has occurred and polling continues, but it MUST NOT satisfy reachability. The harmless post-quantum key-exchange warning emitted on SSH stderr MUST NOT by itself be treated as a probe failure.

#### Scenario: Probe succeeds after reboot progress is observed

- **WHEN** SSH connection is established, the boot-completion probe returns exit code 0, and at least one earlier probe in the same reboot wait failed
- **THEN** the host is considered reachable

#### Scenario: Probe succeeds before reboot progress is observed

- **WHEN** SSH connection is established and the boot-completion probe returns exit code 0 before any earlier probe in the same reboot wait has failed
- **THEN** the probe is treated as evidence that the old session may still be up and polling continues

#### Scenario: Connected but still booting

- **WHEN** SSH connection is established but the login or probe is rejected because the system is still booting (`pam_nologin`)
- **THEN** poll records that reboot progress has been observed, does not satisfy reachability, waits for the next interval, and retries

#### Scenario: Probe connection refused

- **WHEN** SSH connection is refused or the probe otherwise fails while the execution is waiting for reboot completion
- **THEN** poll records that reboot progress has been observed, waits for the next interval, and retries

#### Scenario: Probe timeout

- **WHEN** SSH connection hangs or times out while the execution is waiting for reboot completion
- **THEN** poll records that reboot progress has been observed, waits for the next interval, and retries

#### Scenario: Post-quantum warning on stderr is not a failure

- **WHEN** a probe connection emits the post-quantum key-exchange warning on stderr but the boot-completion check returns exit code 0
- **THEN** the warning is ignored and the probe result is determined solely by the boot-completion check

### Requirement: Configurable poll parameters

The SSH poll interval and overall timeout MUST be configurable via environment variables `SSH_POLL_INTERVAL` (default "10s") and `SSH_POLL_TIMEOUT` (default "10m").

#### Scenario: Custom interval

- **WHEN** `SSH_POLL_INTERVAL=5s` is set
- **THEN** the orchestrator polls every 5 seconds instead of the default 10

#### Scenario: Custom timeout

- **WHEN** `SSH_POLL_TIMEOUT=15m` is set
- **THEN** the orchestrator allows up to 15 minutes before declaring failure

### Requirement: Configured SSH runner transport

The daemon SHALL use a configured SSH runner transport for reboot and reachability operations. That transport MUST apply the configured `SSH_USER`, `SSH_PORT`, `SSH_OPTIONS`, and `SSH_PRIVATE_KEY_PATH` values to both the `reboot` command and the post-reboot boot-completion probe. The transport MUST render the remote command payload from `Command` and `Args` only; `execution_id`, `step_name`, and other workflow metadata MAY be used for local correlation and logging but MUST NOT be appended to the remote command line.

#### Scenario: Reboot command uses configured identity

- **WHEN** an execution reaches the reboot step and the daemon has `SSH_USER=slurm`, `SSH_PORT=2222`, `SSH_PRIVATE_KEY_PATH=/run/secrets/node-key`, and `SSH_OPTIONS=StrictHostKeyChecking=accept-new ConnectTimeout=5`
- **THEN** the daemon issues the `reboot` command through the SSH runner using target `slurm@<host>`, port `2222`, the configured private key file, the configured SSH options, and a rendered remote command of exactly `reboot`

#### Scenario: Reachability probe uses the same transport

- **WHEN** an execution is in state `rebooting` and the orchestrator polls host reachability
- **THEN** each boot-completion probe uses the same configured SSH user, port, private key file, and SSH options as the reboot command

#### Scenario: Workflow metadata does not mutate the remote payload

- **WHEN** the daemon dispatches a reboot or reachability probe with `execution_id` and `step_name` metadata
- **THEN** that metadata is retained only for local correlation and does not appear in the remote command sent over SSH

