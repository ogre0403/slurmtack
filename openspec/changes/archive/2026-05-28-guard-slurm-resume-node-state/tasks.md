## 1. Guarded Attach Logic

- [x] 1.1 Add shared Slurm attach-state classification that recognizes resumable drain/down tokens, already-schedulable active states, and unsupported states from composite node-state strings.
- [x] 1.2 Update the OpenStack-to-Slurm attach paths in `internal/orchestrator` and `internal/slurm` to call `GetNodeState` before `ResumeNode` and apply the shared decision logic.

## 2. Regression Coverage

- [x] 2.1 Add focused tests covering composite drain states that must resume, already-active states that must skip resume, and unsupported states that must fail before mutation.
- [x] 2.2 Update any affected fake Slurm clients or integration-style tests so invalid unconditional resume behavior is no longer treated as acceptable.