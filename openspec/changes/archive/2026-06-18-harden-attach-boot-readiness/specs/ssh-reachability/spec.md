## MODIFIED Requirements

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
