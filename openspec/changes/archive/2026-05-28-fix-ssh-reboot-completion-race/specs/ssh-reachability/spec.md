## MODIFIED Requirements

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

The SSH poll SHALL execute a minimal command (`hostname`) to verify the host is responsive. The probe MUST use the existing `remote.Runner` interface with a short per-attempt timeout (5 seconds). A successful probe SHALL satisfy the reboot wait only after the reboot has already been observed through a prior failed probe.

#### Scenario: Probe succeeds after reboot progress is observed

- **WHEN** SSH connection is established, `hostname` returns exit code 0, and at least one earlier probe in the same reboot wait failed
- **THEN** the host is considered reachable

#### Scenario: Probe succeeds before reboot progress is observed

- **WHEN** SSH connection is established and `hostname` returns exit code 0 before any earlier probe in the same reboot wait has failed
- **THEN** the probe is treated as evidence that the old session may still be up and polling continues

#### Scenario: Probe connection refused

- **WHEN** SSH connection is refused or the probe otherwise fails while the execution is waiting for reboot completion
- **THEN** poll records that reboot progress has been observed, waits for the next interval, and retries

#### Scenario: Probe timeout

- **WHEN** SSH connection hangs or times out while the execution is waiting for reboot completion
- **THEN** poll records that reboot progress has been observed, waits for the next interval, and retries