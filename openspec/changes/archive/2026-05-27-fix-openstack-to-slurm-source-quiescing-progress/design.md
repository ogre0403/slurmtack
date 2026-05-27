## Context

The live `openstack_to_slurm` flow already performs the first half of source quiescing: `doQuiesce` disables the Nova compute service and transitions the execution to `source_quiescing`. After that point, the orchestrator currently treats `source_quiescing` as a passive wait state for O2S, even though the design intent is to verify that OpenStack has actually drained the host before moving to `source_detached`.

The existing OpenStack client already exposes the three signals needed for that verification: compute service status, resident instances, and active migrations. The orchestrator also already has a polling control path and timeout handling for long-lived wait states, so this change can be implemented without adding a new daemon, queue, or persisted state.

## Goals / Non-Goals

**Goals:**
- Ensure `openstack_to_slurm` executions automatically progress from `source_quiescing` to `source_detached` once OpenStack quiesce conditions are satisfied.
- Reuse the existing orchestrator tick loop, state machine, timeout handling, and OpenStack client methods.
- Make the O2S wait state observable enough to distinguish "still draining" from "quiesce satisfied" and from hard errors.

**Non-Goals:**
- Change the `slurm_to_openstack` MQ-driven drain flow.
- Introduce new REST, MQ, or store schema contracts.
- Redesign the downstream `source_detached` to `completed` portion of the workflow.

## Decisions

### Add a dedicated O2S quiesce-verification action

`determineO2S` should stop returning `actionNone` for `source_quiescing` and instead select a dedicated verification action. That keeps the control path explicit in the state-to-action mapping, preserves per-tick retry semantics, and makes the gap visible in tests and traces.

Alternative considered: re-run `doQuiesce` or embed verification into the initial disable step. Rejected because `doQuiesce` only runs once before the execution enters `source_quiescing`, so it cannot drive later progress without overloading one action with two different lifecycle phases.

### Treat quiesce verification as a repeated read-only check

The verification action should query OpenStack on each tick and gate the transition on all of the following being true:
- the compute service for the host is disabled
- the host has no resident instances
- the host has no active migrations

If any condition is still unmet, the action should leave the execution in `source_quiescing` and emit progress logging rather than failing or transitioning. When all conditions are met, it should emit a satisfied log and transition to `source_detached`.

Alternative considered: add a new OpenStack event or background poller. Rejected because the orchestrator already owns active execution progression, and introducing a second control path would duplicate timeout, logging, and concurrency behavior.

### Reuse the current OpenStack client surface

Implementation should compose `GetComputeService`, `ListInstances`, and `ListActiveMigrations` instead of expanding the public client interface. If the code benefits from a helper, that helper should live inside the orchestrator package and aggregate existing client calls into a single quiesce status decision.

Alternative considered: add a new `IsHostQuiesced` client method. Rejected for now because it would shift orchestration policy into the client without adding new external capability.

### Keep hard query failures on the normal step-failure path

A failed OpenStack query should be returned as an action error so the existing orchestrator failure classification handles it as a pre-mutation failure from `source_quiescing`. This keeps the behavior consistent with other orchestrator steps, while the existing timeout checker continues to cover the "not yet quiesced" case where the checks are succeeding but the host is still draining.

Alternative considered: silently retry API errors forever. Rejected because it would blur operator-visible control-plane failures into the same bucket as legitimate draining delay.

## Risks / Trade-offs

- Transient Nova read errors could fail an execution earlier than a pure retry policy would. → Mitigation: keep the logic narrow, surface the failing operation in logs/tests, and revisit retry semantics only if real telemetry shows noisy failures.
- Each active O2S execution in `source_quiescing` adds repeated OpenStack reads on every tick. → Mitigation: reuse the existing tick interval and rely on per-node lease serialization to keep concurrency low.
- The check intentionally treats any resident instance or active migration as "not quiesced," which may delay progress in edge cases where an instance is present but already non-actionable. → Mitigation: prefer safety first and refine status filtering later with evidence.

## Migration Plan

No schema or API migration is required. After deployment, newly created executions and any in-flight `openstack_to_slurm` executions already parked in `source_quiescing` will be eligible to advance on the next orchestrator tick. Rollback is a normal code rollback; the prior behavior would resume and affected executions would remain in `source_quiescing` until timeout or manual intervention.

## Open Questions

- None for this change.