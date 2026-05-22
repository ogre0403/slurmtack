## 1. Execution Foundation

- [x] 1.1 Create Go packages for execution records, step records, node leases, and state-transition enums.
- [x] 1.2 Implement a persistence interface for creating executions, advancing versioned states, and acquiring or releasing per-node leases.
- [x] 1.3 Add request acceptance and status-query entrypoints so a switch request returns an execution ID immediately.

## 2. Control-Plane Workflow

- [x] 2.1 Implement the shared switch state machine runner with validation for legal transitions and terminal failure classification.
- [x] 2.2 Implement the Slurm placeholder-job path that submits the allocation request, records the placeholder job ID, and binds the execution when a matching allocation event arrives.
- [x] 2.3 Implement the OpenStack-to-Slurm precheck path that blocks switching when instances or in-flight compute operations still exist.
- [x] 2.4 Implement source-quiesce, source-detach, target-attach, and verification step interfaces so each direction can plug in source and target specific handlers.

## 3. Remote Runner And Event Safety

- [x] 3.1 Implement the SSH command-wrapper runner that tags each invocation with `execution_id` and `step_name` and captures structured results.
- [x] 3.2 Implement version-checked message handling for allocation, drained, and host-reachable events so stale or duplicate events are ignored.
- [x] 3.3 Add timeout and retry handling for allocation wait, drain wait, reboot wait, and verification states.

## 4. Observability And Recovery

- [x] 4.1 Implement execution and step evidence writing under `/var/log/gpu-switch/<node_name>/<execution_id>/`, including manifest and event stream files.
- [x] 4.2 Implement snapshot and diagnostics capture for pre-mutation, post-verification, and reboot-boundary evidence.
- [x] 4.3 Implement compensation hooks and terminal-state handling for `failed_non_destructive`, `failed_needs_rollback`, and `failed_manual_recovery`.

## 5. Verification

- [x] 5.1 Add unit tests for state transition validation, lease exclusivity, and duplicate-event rejection.
- [x] 5.2 Add integration-style tests with fake Slurm, OpenStack, and remote-runner adapters covering both switch directions.
- [x] 5.3 Add a dry-run or simulated execution path that exercises logging, evidence capture, and status reporting without mutating a real host.