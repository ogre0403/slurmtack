## 1. Remote transport contract

- [x] 1.1 Update `internal/remote` so the SSH runner and executor preserve `Command` plus `Args` as the exact remote payload and stop synthesizing `--execution-id` or `--step-name` arguments.
- [x] 1.2 Add a shared SSH rendering path that produces the resolved target and rendered remote command for both execution and tests.

## 2. Correlated SSH dispatch logging

- [x] 2.1 Emit a structured pre-dispatch log for SSH commands that includes the target host, rendered remote command, and available `execution_id` or `step_name` metadata while excluding local SSH transport details.
- [x] 2.2 Thread execution metadata into the post-reboot `hostname` probe so reboot and reachability dispatch logs stay attributable to the same execution.

## 3. Focused verification

- [x] 3.1 Add or update tests for reboot invocation, reachability probe payload integrity, and SSH dispatch logging in `internal/remote` and `internal/orchestrator`.
- [x] 3.2 Run focused Go tests for the touched packages and confirm the change is ready for implementation.