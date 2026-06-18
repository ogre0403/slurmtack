## 1. State Machine Alignment

- [x] 1.1 Allow `host_reachable` to transition to the terminal failed state used for attach-stage mutation failures.
- [x] 1.2 Keep attach-step failure handling routed through the existing mutation-partial classification so `host_reachable` errors no longer leave executions active.

## 2. Regression Coverage

- [x] 2.1 Add state-machine or runner coverage proving `FailExecution` can terminate an execution from `host_reachable`.
- [x] 2.2 Add orchestrator regression tests for `openstack_to_slurm` attach failures from `host_reachable` ending in `failed_needs_rollback`.
- [x] 2.3 Add orchestrator regression tests for `slurm_to_openstack` attach failures from `host_reachable` ending in `failed_needs_rollback`.
