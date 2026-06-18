## Context

The orchestrator uses `classifyFailure` to map errors before source detachment to `failed_non_destructive`, errors after reboot uncertainty to `failed_manual_recovery`, and all other post-mutation failures to `failed_needs_rollback`. Both switch directions run target attachment from `host_reachable`, so an attach error at that point is already in the post-mutation phase and should terminate the execution instead of leaving it active.

Today the classification logic and the persisted transition graph disagree: `FailExecution` chooses `failed_needs_rollback` for attach failures from `host_reachable`, but `allowedTransitions` does not permit that edge. The result is that a real attach error is logged while the execution remains stuck in `host_reachable`.

## Goals / Non-Goals

**Goals:**

- Make attach failures from `host_reachable` reach a durable terminal failed state.
- Keep failure behavior consistent between `openstack_to_slurm` and `slurm_to_openstack`.
- Add regression coverage around the invalid transition that currently leaves executions active.

**Non-Goals:**

- Redesign the broader failure taxonomy or compensation workflow.
- Change the meaning of `failed_manual_recovery` for reboot reachability timeouts.
- Introduce new workflow states for attach retries or partial target recovery.

## Decisions

### Treat `host_reachable` as a post-mutation state that may fail with rollback needed

`host_reachable` happens only after source detachment, host reconfiguration, and reboot handling. An attach failure from this state is therefore still a mutation-partial failure and should be allowed to terminate as `failed_needs_rollback`.

Alternative considered: remap `host_reachable` failures to `failed_manual_recovery`.
This was rejected because many attach failures are ordinary target-side errors, not unknown host-state situations, and escalating all of them to manual recovery would make the failure model less precise.

### Preserve failure classification and fix the transition contract

The current failure classifier already routes `host_reachable` failures to `FailureMutationPartial`. The minimal fix is to align the transition graph with that contract by allowing terminal failure from `host_reachable`.

Alternative considered: change `classifyFailure` or `doAttach` to special-case pre-transition attach errors.
This was rejected because the bug is not in step classification; it is in the state machine refusing a terminal transition that the orchestrator intentionally chose.

### Cover both direction-specific attach paths with regression tests

`openstack_to_slurm` can fail during slurmd restore or Slurm attach readiness checks, while `slurm_to_openstack` can fail during the OpenStack compute-service enable path. Regression tests need to prove that either branch leaves the execution in a failed terminal state rather than `host_reachable`.

Alternative considered: only add a state-transition unit test.
This was rejected because the observed bug is user-visible through orchestrator behavior, and regression coverage should protect both the transition primitive and the real attach action entry points.

## Risks / Trade-offs

- Visible terminal-state change for attach failures -> API and UI consumers will now see `failed_needs_rollback` where they previously saw an indefinitely active execution. This is the intended behavior, but downstream tooling may have been compensating for the bug.
- Broader reachability of `failed_needs_rollback` from `host_reachable` -> Future handlers that fail from `host_reachable` will also be able to terminate. This is acceptable because `host_reachable` is already beyond the non-destructive phase.

## Migration Plan

No data migration is required. Deploy the state-machine and orchestrator test updates together so new executions terminate correctly. Existing stuck executions will remain in their persisted state until manually reconciled.

## Open Questions

None.
