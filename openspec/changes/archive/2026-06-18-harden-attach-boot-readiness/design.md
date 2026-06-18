## Context

The `openstack_to_slurm` flow ends with: `rebooting → doSSHPoll → host_reachable → doAttach`.

- `doSSHPoll` (`internal/orchestrator/reachability.go`) polls with `PollSSHReachable`, which runs `hostname` and accepts the host as reachable on the first exit-0 probe that occurs *after* a prior failed probe (the reboot-observed gate).
- `doAttach` (`internal/orchestrator/orchestrator.go`) then runs `runSlurmdServiceCommands(ctx, exec, "enable", "start")`, each a single SSH `systemctl` call with no retry.

The problem: sshd starts accepting connections before the system finishes booting. During that window `pam_nologin` rejects unprivileged sessions ("System is booting up. Unprivileged users are not permitted to log in yet."). So `hostname` can succeed (or the gap between probe and attach is enough), and the `systemctl start slurmd` call lands inside the `pam_nologin` window and fails. Because `ExecSSHExecutor.Run` returns `exitCode = -1` for a session terminated this way (`ssh_executor.go:61-65` only maps `*exec.ExitError`; a PAM-aborted session surfaces as a non-ExitError or a -1 exit), the orchestrator reports `slurmd start failed: exit code -1: <stderr>`, where stderr also carries the unrelated post-quantum warning banner.

## Goals / Non-Goals

**Goals:**
- Make `host_reachable` mean "the node finished booting", closing the `pam_nologin` window before attach runs.
- Make the slurmd restore commands resilient to a residual boot-window race so the execution does not fail spuriously.
- Avoid misreading the post-quantum SSH stderr banner as a failure.

**Non-Goals:**
- Changing the reboot-observed two-phase poll semantics (must still require a failed probe before accepting success).
- Adding new required configuration. Reuse `SSH_POLL_INTERVAL` / `SSH_POLL_TIMEOUT`.
- Touching the `slurm_to_openstack` stop/disable path (it runs before reboot, not after).

## Decisions

### Probe checks boot completion, not just connectivity
Change the probe command from `hostname` to a boot-completion check. Preferred: `test ! -f /run/nologin` — this maps directly to the `pam_nologin` mechanism causing the failure, is universally available, and needs no systemd assumptions. Considered `systemctl is-system-running` (accept `running`/`degraded`) but rejected as primary because `degraded` handling is fuzzy and it assumes systemd query availability; `/run/nologin` is the precise gate.

The probe must still distinguish three outcomes:
1. **Connected + boot complete** → exit 0 → satisfies reachability (after reboot observed).
2. **Connected but still booting** (`/run/nologin` present → `test` exits non-zero, or login rejected by `pam_nologin`) → counts as reboot progress, keep polling, do NOT satisfy.
3. **Connection failure** (refused/timeout) → reboot progress, keep polling.

`classifyProbeResult` will be extended: a non-zero exit whose message indicates booting (`pam_nologin` / "System is booting up") is classified as "not ready" rather than a hard error, and folded into the same "progress observed, keep polling" branch. The post-quantum banner on stderr is ignored when exit code is 0.

### Bounded retry on slurmd restore for boot-transient errors
Add a small retry wrapper around `runSlurmdServiceCommand` used only by the OpenStack-to-Slurm enable/start path. Retry only when the failure is boot-transient (stderr/stdout contains `pam_nologin` / "System is booting up", or the SSH session closed during login). Use a few attempts with short backoff bounded well under `slurmdCommandTimeout`. Non-transient failures fail immediately as today. This is defense-in-depth for the narrow race between a passing probe and the attach call.

### Classify exit -1 + transient message correctly
Keep returning the result with the message, but the retry classifier inspects the message text (not just the exit code) to decide transient vs. terminal, so `exit code -1: ...pam_nologin...` is retried rather than failing the execution.

## Risks / Trade-offs

- **`/run/nologin` may be removed slightly before all services (incl. slurmd's deps) are ready** → Mitigation: the bounded retry on the slurmd commands absorbs the residual gap; the two mechanisms together cover the window.
- **Longer wall-clock before `host_reachable`** since we wait for true boot completion → acceptable; bounded by existing `SSH_POLL_TIMEOUT` (default 10m).
- **Probe command portability** (`test`/`/run/nologin` assume Linux + PAM) → matches the target environment (Slurm compute nodes); documented as an assumption.
- **Retry could mask a genuinely broken slurmd** → Mitigation: retries are bounded and only triggered by boot-transient message patterns; other failures fail fast.

## Migration Plan

No data or config migration. Behavior change only. Rollback is reverting the probe command and removing the retry wrapper. Existing `reachability_test.go` and `attach_guard_test.go` expectations (which assert `hostname` and single `systemctl` calls) must be updated alongside the implementation.

## Open Questions

- Should the boot-transient retry bound be hard-coded or exposed via env? Default to hard-coded constant (e.g. matching the poll interval style) unless a need for tuning emerges.
