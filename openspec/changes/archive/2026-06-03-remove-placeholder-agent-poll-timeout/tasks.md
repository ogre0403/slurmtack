## 1. Placeholder Agent Polling

- [x] 1.1 Remove `POLL_TIMEOUT` parsing, deadline handling, and the dedicated timeout exit path from `cmd/placeholder-agent/main.go`
- [x] 1.2 Implement token-based drain-state classification so composite states like `MIXED+DRAIN`, `drained+drain`, and `down+drain` finish polling
- [x] 1.3 Keep the existing Slurm poll interval and MQ event flow intact while ensuring the agent now waits until drain completion or external Slurm/job termination

## 2. Regression Coverage

- [x] 2.1 Update `cmd/placeholder-agent/main_test.go` to remove timeout-based expectations and add coverage for composite drain states, especially the `MIXED+DRAIN` regression from `slurm-14.out`
- [x] 2.2 Update any integration or packaging-adjacent tests/docs that still refer to `POLL_TIMEOUT` or exit code 2
- [x] 2.3 Run focused tests for `cmd/placeholder-agent` and any affected Slurm state helpers
