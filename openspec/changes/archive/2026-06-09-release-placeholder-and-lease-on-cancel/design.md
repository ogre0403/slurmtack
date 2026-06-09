## Context

The current cancellation model persists `cancellation_source_state` when an operator claims cancel, then the orchestrator chooses cleanup actions from that state. In practice, `slurm_to_openstack` can cross asynchronous boundaries between the cancel claim and the cleanup step: a placeholder job may already exist, a node may already be bound, and the datastore may already contain a lease for that execution. The current source-state-driven cleanup path is therefore too narrow for the observed bug, because `awaiting_source_allocation` cleanup cancels the placeholder job but does not guarantee that any execution-owned lease record is removed.

This change is cross-cutting because the bug sits at the boundary between API/service-side cancel admission, orchestrator cleanup, async allocation timing, and persisted lease state.

## Goals / Non-Goals

**Goals:**

- Ensure a successfully cancelled `slurm_to_openstack` execution leaves no execution-owned placeholder job running.
- Ensure a successfully cancelled execution leaves no execution-owned lease record in the datastore.
- Keep cancellation cleanup idempotent so repeated processing or partial prior cleanup does not block terminal cancellation.
- Preserve the existing source-state-specific rollback behavior, such as resuming a Slurm node only when cancellation happens after source quiesce has begun.

**Non-Goals:**

- Expanding which execution states are cancellable.
- Changing the normal non-cancel switch path or the placeholder-agent lifecycle outside cancellation handling.
- Introducing a background garbage collector for leaked jobs or leases unrelated to an explicit cancellation flow.

## Decisions

### 1. Separate source rollback from resource cleanup

`cancellation_source_state` will continue to decide whether the system must roll source ownership back, such as `ResumeNode` for `slurm_to_openstack` in `source_quiescing` or `EnableComputeService` for `openstack_to_slurm`. Placeholder job cancellation and lease release, however, should be driven by the current execution snapshot: if `placeholder_job_id` is present, cancel it; if the execution still owns a lease for `node_name`, release it.

This keeps the existing state model intact while closing the gap where resources are already attached even though cancellation was accepted from an earlier wait state.

Alternative considered: keep the current switch-on-source-state logic and add more special-case branches for individual races. Rejected because it hard-codes known races instead of expressing the invariant that cancelled executions must not retain owned external resources.

### 2. Treat missing cleanup targets as already-cleaned, not as cancellation failures

Cancellation cleanup should remain safe to retry. If the placeholder job is already gone or the lease record is already absent, cleanup should continue toward `cancelled` rather than converting the execution into a terminal cleanup failure. Genuine API or store errors should still fail the cleanup step.

Alternative considered: fail whenever cancel-job or release-lease does not find a target. Rejected because a retry, concurrent operator action, or earlier partial cleanup would then turn a logically safe cancellation into an unnecessary failure.

### 3. Read resource ownership from the execution at cleanup time

The cleanup step should evaluate the latest persisted execution fields and lease ownership when it runs, instead of relying only on the fields that were present when cancel was first claimed. This matches the system's asynchronous design, where allocation events and orchestrator advancement can occur near the same boundary as a cancel request.

Alternative considered: persist a separate cleanup snapshot during `CancelSwitch`. Rejected because it adds more state to maintain and still risks drift from the actual persisted execution and lease tables.

## Risks / Trade-offs

- [Late async events race with cancellation] -> Mitigation: preserve the existing guard that rejects normal progression from `cancelling`, and extend tests to cover cancellation after allocation-related data has already been persisted.
- [Idempotent cleanup may mask whether a resource ever existed] -> Mitigation: keep existing cancellation tracing and add explicit tests for job-present, lease-present, and already-absent cleanup cases.
- [Cleanup logic now spans both state and resource ownership] -> Mitigation: keep the implementation structured in two phases so rollback actions and generic resource teardown stay easy to reason about.

## Migration Plan

No schema migration is required. Roll out the orchestrator cleanup change together with regression tests. Rollback is a code revert of the cancellation cleanup refactor if needed.

## Open Questions

None.
