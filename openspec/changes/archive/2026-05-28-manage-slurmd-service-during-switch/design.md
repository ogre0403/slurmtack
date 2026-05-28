## Context

The current workflow already uses the Slurm API to drain nodes during `slurm_to_openstack` and to resume nodes during `openstack_to_slurm`, but it does not restore or quiesce the `slurmd` daemon around those control-plane calls. In the implementation, `internal/orchestrator/orchestrator.go` drains the node in `doQuiesce`, moves directly into host reconfiguration from `doReconfigure`, and calls `slurm.EnsureNodeReadyForAttach` from `doAttach` without any SSH-backed `slurmd` lifecycle step.

The codebase already has the primitives needed to add this behavior without new infrastructure: `remote.Runner` executes commands over SSH with workflow metadata, and the orchestrator already depends on it for reboot handling.

## Goals / Non-Goals

**Goals:**
- Enforce `slurmd` shutdown after Slurm drain and before host-side mutation in `slurm_to_openstack`.
- Enforce `slurmd` enable/start before any Slurm attach or resume logic in `openstack_to_slurm`.
- Fail the workflow immediately when service control fails, rather than continuing with a partially attached or partially detached node.
- Reuse the current state machine and SSH runner abstractions.

**Non-Goals:**
- Changing the persisted state model or adding new switch states.
- Managing services other than `slurmd`.
- Reworking Slurm drain/resume semantics or OpenStack compute-service handling.

## Decisions

### Run Slurm-to-OpenStack service shutdown in `doReconfigure`

For `slurm_to_openstack`, the orchestrator will execute `systemctl stop slurmd` followed by `systemctl disable slurmd` through the configured SSH runner before it transitions into `host_reconfiguring`.

This keeps control-plane quiesce (`DrainNode`) and node-local mutation (`systemctl`) separated at the existing state boundary. The alternative was adding a new dedicated state between `source_detached` and `host_reconfiguring`, but that would enlarge the state machine for a short, single-host action that fits the current reconfiguration phase.

### Run OpenStack-to-Slurm service restore in `doAttach`

For `openstack_to_slurm`, the orchestrator will execute `systemctl enable slurmd` followed by `systemctl start slurmd` before it evaluates Slurm attach state and before it calls `ResumeNode` when resume is required.

This ordering ensures the host-side daemon is prepared even when `slurm.EnsureNodeReadyForAttach` short-circuits because the node is already in an attachable Slurm state. The alternative was extending `slurm.EnsureNodeReadyForAttach` to manage SSH-backed service actions, but that helper currently owns only Slurm API state evaluation and should not grow a transport dependency.

### Keep service commands explicit and sequential

The service lifecycle will use two explicit SSH commands per direction instead of a single compound shell command.

This produces clearer trace/debug output, makes it obvious which command failed, and keeps recovery guidance specific. The alternative was using a combined command such as `systemctl disable --now slurmd` or `systemctl enable --now slurmd`, but that would hide which sub-action failed and reduce test precision.

## Risks / Trade-offs

- [SSH runner becomes mandatory for these handoff paths] -> Mitigation: fail early with the existing `ssh runner not configured` error path and cover that behavior in orchestrator tests.
- [A partial service mutation can leave `slurmd` stopped or disabled after a failed switch] -> Mitigation: classify the failure in the existing mutation failure path and surface the failed command for manual recovery.
- [Attach-state logic can still skip `ResumeNode`] -> Mitigation: restore `slurmd` before attach-state evaluation so service readiness does not depend on whether Slurm needs an API-side resume.

## Migration Plan

No schema or data migration is required. Rollout consists of shipping the orchestrator change together with updated unit and integration coverage. Rollback is a code revert of the SSH-backed `slurmd` steps if an environment cannot yet provide SSH runner configuration.

## Open Questions

No open questions remain for proposal-level implementation. The change can proceed with the existing SSH runner contract and current state machine.