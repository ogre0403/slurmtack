## 1. Rework reboot polling

- [x] 1.1 Update `PollSSHReachable` to track whether reboot progress has been observed and only return success after a failed probe is followed by a later successful probe.
- [x] 1.2 Preserve the existing timeout and failure-class behavior for both "never became unreachable" and "became unreachable but never returned" cases, and emit trace/log context that distinguishes ignored early successes from post-reboot recovery.

## 2. Add regression coverage

- [x] 2.1 Extend the reboot reachability tests with a stateful fake runner that covers success-before-failure-success probe sequences and verifies the execution stays in `rebooting` until the post-failure success.
- [x] 2.2 Add timeout-oriented tests for hosts that never become unreachable after `reboot` and for hosts that become unreachable but do not come back before the poll deadline.

## 3. Validate the change

- [x] 3.1 Run the focused Go test suites for the orchestrator reachability path and any directly affected polling helpers.
- [x] 3.2 Verify the implementation and trace output still match the updated `ssh-reachability` spec and the design decisions in this change.