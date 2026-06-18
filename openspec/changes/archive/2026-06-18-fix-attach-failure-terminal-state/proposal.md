## Why

Attach-step failures that happen after the host becomes SSH-reachable are currently classified as post-mutation failures, but the persisted state machine does not allow the execution to leave `host_reachable` for the expected terminal failure state. In practice this leaves the execution `active` and waiting forever after a real attach failure, which hides the actual outcome from operators.

This has already been observed in `openstack_to_slurm`, and the same `host_reachable -> attach` control path exists in `slurm_to_openstack`, so the failure contract needs to be made explicit for both directions.

## What Changes

- Allow attach-stage failures from `host_reachable` to transition into a terminal failed state instead of remaining `active`.
- Define the expected failure behavior for target-attach errors after reboot reachability in both `openstack_to_slurm` and `slurm_to_openstack`.
- Add regression coverage for attach failures that occur before `target_attaching` is persisted, including direction-specific attach guard failures.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `gpu-node-switch-orchestration`: clarify that attach failures after `host_reachable` must terminate the execution with a durable failed outcome instead of leaving it in a non-terminal waiting state.

## Impact

- Affected code: `internal/domain/transitions.go`, orchestrator failure handling, and attach-path tests.
- Affected behavior: execution terminal status after attach-step failures in both switch directions.
- Operational impact: API and UI consumers will observe failed executions instead of indefinitely active ones when target attachment cannot proceed.
