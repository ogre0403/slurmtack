## 1. Orchestrator Quiesce Progression

- [x] 1.1 Add a dedicated `openstack_to_slurm` action for executions in `source_quiescing` and update the O2S state-to-action mapping to schedule it on each tick.
- [x] 1.2 Implement the O2S quiesce verification path in `internal/orchestrator` by reading compute-service status, resident instances, and active migrations, leaving the execution in `source_quiescing` until all exit conditions are satisfied.
- [x] 1.3 Transition verified O2S executions from `source_quiescing` to `source_detached` and emit clear wait-progress or wait-satisfied trace events for the verification loop.

## 2. Regression Coverage

- [x] 2.1 Add focused orchestrator unit tests for O2S `source_quiescing` covering "still draining", "verification succeeded", and "OpenStack query failed" outcomes.
- [x] 2.2 Update or add orchestrator-facing tests to confirm the live control path no longer stalls in O2S `source_quiescing` and that existing failure handling remains intact.