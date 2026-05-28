## Why

The `openstack_to_slurm` attach path currently calls Slurm `RESUME` unconditionally. In real Slurm, `RESUME` is only valid when the node is in a resumable state such as drain or down, so issuing it against an already active node can fail with `slurm_update error: Invalid node state specified` and unnecessarily fail the switch.

## What Changes

- Require the OpenStack-to-Slurm target-attach flow to read the current Slurm node state before deciding whether to call `ResumeNode`.
- Define resumable versus non-resumable Slurm node states for the attach step, including composite drain/down states returned by `GetNodeState`.
- Make attach behavior safe for nodes that are already schedulable by skipping the `RESUME` mutation instead of surfacing an avoidable Slurm API error.
- Add focused regression coverage for guarded resume behavior in the attach path.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `gpu-node-switch-orchestration`: `openstack_to_slurm` target attachment must only issue Slurm `RESUME` when the node is currently in a resumable drain/down state, and must avoid failing the workflow on already-active node states that do not require resume.

## Impact

- Affected code: `internal/orchestrator`, `internal/slurm`, and targeted tests around O2S attach behavior.
- External systems: Slurm node-state reads become an explicit precondition for the attach mutation.
- APIs: no REST, MQ, or store schema changes are expected.