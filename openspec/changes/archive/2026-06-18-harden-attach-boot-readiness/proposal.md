## Why

In the `openstack_to_slurm` flow, after the target node reboots the orchestrator declares the host "reachable" as soon as a single `hostname` probe succeeds. But an SSH session succeeding does not mean the node has finished booting: `pam_nologin` still rejects unprivileged logins while `/run/nologin` exists. As a result `doAttach` runs `systemctl enable/start slurmd` against a node that is still booting and fails with `System is booting up. Unprivileged users are not permitted to log in yet`, surfaced as `slurmd start failed: exit code -1`. This is a real, reproducible failure on the FUSION-03 worker test.

## What Changes

- Strengthen the post-reboot SSH probe so reachability means **boot complete**, not merely "sshd answered". The probe MUST confirm the node is no longer in the `pam_nologin` window (e.g. absence of `/run/nologin`) before the orchestrator transitions to `host_reachable`.
- Treat a probe that connects but is rejected because the system is still booting as "not ready yet — keep polling", distinct from a hard probe failure, so the existing reboot-observed logic is preserved.
- Make the OpenStack-to-Slurm slurmd restore (`systemctl enable/start slurmd`) tolerant of boot-transient errors: a small bounded retry/backoff so a residual `pam_nologin` race between probe success and attach does not fail the whole execution.
- Keep the harmless post-quantum SSH banner (`connection is not using a post-quantum key exchange algorithm`) from being misread as a failure — it appears on stderr of every connection and MUST NOT by itself mark a step failed.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `ssh-reachability`: the post-reboot probe definition changes from "`hostname` exit 0" to "host has finished booting (out of the `pam_nologin` window)", and adds a probe disposition for "connected but still booting → keep polling".
- `gpu-node-switch-orchestration`: the "Slurmd is restored before OpenStack-to-Slurm attachment" requirement gains bounded retry on boot-transient SSH failures before the step is declared failed.

## Impact

- `internal/orchestrator/reachability.go`: probe command and result classification.
- `internal/orchestrator/orchestrator.go`: `runSlurmdServiceCommand` retry behavior; possibly probe step wiring.
- Tests: `internal/orchestrator/reachability_test.go`, `internal/orchestrator/attach_guard_test.go`, and related attach/reachability tests.
- No config schema change required; reuses `SSH_POLL_INTERVAL`/`SSH_POLL_TIMEOUT`. A new optional retry bound MAY be introduced.
