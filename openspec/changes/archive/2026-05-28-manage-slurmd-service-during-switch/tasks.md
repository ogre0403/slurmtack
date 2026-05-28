## 1. Slurmd service control helpers

- [x] 1.1 Add a small helper around `remote.Runner` execution to run `systemctl stop/disable/start/enable slurmd` with explicit step names, timeouts, and error wrapping.
- [x] 1.2 Reuse the existing SSH-runner configuration path so missing runner configuration or command failures surface as actionable orchestration errors.

## 2. Workflow sequencing changes

- [x] 2.1 Update the `slurm_to_openstack` path in `internal/orchestrator/orchestrator.go` so the daemon stops and disables `slurmd` after Slurm drain succeeds and before it transitions into host reconfiguration.
- [x] 2.2 Update the `openstack_to_slurm` path in `internal/orchestrator/orchestrator.go` so the daemon enables and starts `slurmd` before it evaluates Slurm attach state or issues `ResumeNode`.
- [x] 2.3 Preserve the existing attach-state guard behavior after service restore so `ResumeNode` remains conditional on the current Slurm node state.

## 3. Tests and validation

- [x] 3.1 Add or extend orchestrator tests to verify SSH command ordering for both directions and to assert that service-control failures block the workflow.
- [x] 3.2 Extend attach/quiesce-facing tests or fakes as needed so the new `slurmd` steps are covered without weakening existing drain/resume assertions.
- [x] 3.3 Run focused Go tests for the touched orchestrator, slurm, and remote runner paths.