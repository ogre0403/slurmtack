## Context

`PollSSHReachable` currently runs a single loop that executes `hostname` until one probe succeeds or the overall timeout expires. The orchestrator enters `rebooting` immediately after dispatching `reboot`, and the existing tests encode that a single successful probe is enough to transition to `host_reachable`.

That behavior is racy for real reboot flows. After the reboot command is accepted, the old OS can still answer SSH briefly while shutdown scripts are running. A successful `hostname` in that window does not prove that the machine has left the old boot or started the new one.

## Goals / Non-Goals

**Goals:**
- Require positive evidence that the host left the pre-reboot SSH-reachable state before the orchestrator can declare reboot completion.
- Preserve the existing `remote.Runner` abstraction, poll interval, and overall timeout configuration.
- Add deterministic tests for success-before-failure and failure-then-success probe sequences.

**Non-Goals:**
- Changing how the reboot command itself is dispatched.
- Adding new external services, persistent state, or database schema.
- Solving every possible node liveness ambiguity beyond this reboot race.

## Decisions

### Use a two-phase reboot wait inside SSH polling

The reachability loop will track whether reboot progress has been observed. While the host is still responding successfully and no failed probe has been seen, the poll stays in a "waiting for reboot to start" phase and ignores those early successes. Once a probe fails, the poll moves to a "waiting for host to return" phase. Only a later successful probe in that second phase satisfies the reboot wait.

This is the smallest change that fixes the race at the point where the wrong decision is made today. It avoids adding new orchestrator states or changing the store contract.

Alternative considered: add a fixed delay before polling. This was rejected because it turns the race into a timing guess and either still fails on slow shutdowns or adds unnecessary delay on fast reboots.

Alternative considered: compare a boot identifier before and after reboot. This was rejected for now because it would require pre-reboot capture, a broader API change to carry that value, and tighter Linux-specific coupling than the current workflow needs.

### Treat probe failure as the reboot-start signal

The implementation will treat a failed SSH probe after `reboot` as evidence that the old SSH-reachable session is gone. That keeps the change local to `PollSSHReachable` and works with the current `remote.Runner` API, which already collapses transport and command failures into an error.

Alternative considered: require typed connection errors or multiple consecutive failures before accepting that reboot started. This was rejected because the current runner does not expose typed failure categories, and extra thresholds add complexity without addressing the core false-positive path described in the proposal.

### Keep one overall timeout budget

The existing `SSH_POLL_TIMEOUT` budget will continue to cover the full wait, including both "waiting for reboot to start" and "waiting for host to return" phases. This keeps configuration stable and preserves the current failure class when reboot confirmation cannot be established in time.

Alternative considered: split the timeout into separate "wait for disconnect" and "wait for reconnect" budgets. This was rejected because it introduces new configuration and tuning burden before there is evidence that operators need phase-specific controls.

### Expand reboot reachability test coverage

The orchestrator tests will use a stateful fake runner so they can model sequences like success-success-failure-success and assert that only the post-failure success advances to `host_reachable`. Timeout tests will also cover the case where the host never becomes unreachable after `reboot`.

## Risks / Trade-offs

- A transient SSH failure unrelated to reboot could satisfy the "reboot started" signal. Mitigation: keep the logic scoped to the immediate post-reboot wait, log phase changes, and cover the intended success/failure sequences with tests.
- Hosts that keep SSH up for a while before shutdown will remain in `rebooting` longer than before. Mitigation: this is intentional because the previous behavior was incorrect; the existing timeout already bounds the wait.
- The change makes the polling logic slightly more stateful. Mitigation: confine the state machine to `PollSSHReachable` and verify behavior with focused unit tests.

## Migration Plan

No data migration or API migration is required. The change can be delivered by updating the polling logic and tests, then rolling out with the existing SSH poll configuration.

## Open Questions

- Whether phase changes should emit new dedicated trace fields or reuse the existing wait progress events can be decided during implementation without affecting the external contract.