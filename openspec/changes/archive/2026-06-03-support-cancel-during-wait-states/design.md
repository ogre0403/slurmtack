## Context

The API already exposes `POST /v1/switches/:id/cancel`, but it is a 501 stub while real executions can sit in long waits such as:

- `awaiting_target_node` while MQ node selection is not correlated
- `awaiting_source_allocation` while the placeholder job is still queued
- `source_quiescing` while the workflow waits for source-side quiesce conditions

Those waits are operationally different from deeper execution states like `rebooting` or `verifying`. In the wait states above, the system is stalled on an external signal or poll result and operators reasonably need a stop button. In later states, ownership mutation or host mutation has already progressed far enough that "cancel" would really mean rollback or manual recovery, not merely "stop waiting."

The current workflow also has concurrency constraints: MQ consumers, the orchestrator, and the API can all touch the same execution. A cancellation design must therefore claim the execution before cleanup starts, otherwise a late MQ event or a successful poll could advance the workflow while the cancel path is trying to tear it down.

## Goals / Non-Goals

**Goals:**

- Implement `/v1/switches/:id/cancel` for both directions.
- Allow cancellation only from a curated set of wait states.
- Make cancellation durable and race-safe against MQ deliveries, poll completions, and daemon restarts.
- Ensure cancellation performs the required cleanup for each wait state before the execution reaches a terminal outcome.

**Non-Goals:**

- Make every active state cancellable.
- Define a general rollback framework for post-detach or post-reboot executions.
- Change MQ message schemas or placeholder-agent behavior.
- Redesign the existing failure-state taxonomy beyond what is needed to represent operator cancellation cleanly.

## Decisions

### 1. Introduce an explicit cancellation lifecycle: `cancelling` -> `cancelled`

Cancellation will be modeled as its own persisted lifecycle instead of reusing `failed_non_destructive` directly.

The API/service path will first claim an eligible execution by transitioning it from its current wait state to `cancelling`. The execution record will also persist the original wait state as cancellation context so the cleanup plan remains recoverable after restart. Once the cleanup action succeeds, the execution transitions to terminal `cancelled`.

This is preferred over a one-step jump straight to a terminal failure because the claim step prevents concurrent normal progression and gives restart recovery an unambiguous state to resume.

Alternatives considered:

- Reuse `failed_non_destructive` as the immediate cancel result. Rejected because it conflates user intent with system failure and does not provide a safe "claimed but not yet cleaned up" state.
- Keep the original wait state and store a separate boolean `cancel_requested`. Rejected because it complicates every state/action matcher and makes it easier for old code paths to ignore the cancellation flag.

### 2. Only allow cancellation from approved wait states

The system will accept cancellation only from:

- `awaiting_target_node`
- `awaiting_source_allocation`
- `source_quiescing`

This covers both directions while the workflow is still stalled at a wait boundary and before it has progressed into post-detach host mutation or ambiguous post-reboot recovery.

The system will reject cancellation from states such as `requested`, `locked`, `precheck_passed`, `source_detached`, `host_reconfiguring`, `rebooting`, `target_attaching`, and `verifying`.

Alternative considered:

- Allow cancellation from all polling states, including `rebooting` and `verifying`. Rejected because those stages occur after deeper workflow mutation and need explicit rollback/manual-recovery semantics rather than a "stop waiting" contract.

### 3. Cleanup is orchestrator-owned and keyed by the captured wait state

After the API claims the execution as `cancelling`, the orchestrator will perform a dedicated cancellation cleanup action chosen from `(direction, cancellation_source_state)`.

Cleanup plan:

- `awaiting_target_node`: no external cleanup; finalize cancellation directly.
- `awaiting_source_allocation`: cancel the placeholder job when `placeholder_job_id` is known.
- `source_quiescing` for `slurm_to_openstack`: resume the Slurm node, cancel the placeholder job when present, and release the node lease.
- `source_quiescing` for `openstack_to_slurm`: re-enable the OpenStack compute service and release the node lease.

This keeps workflow-side cleanup in one place, reuses existing Slurm/OpenStack clients, and allows daemon restart recovery to resume the same action.

Alternative considered:

- Run all cleanup synchronously inside the API handler. Rejected because it couples HTTP latency to external control-plane calls and makes race handling with MQ/orchestrator much harder.

### 4. Successful cancellation gets its own terminal state, but keeps `overall_status=failed`

On successful cleanup, the execution will transition to terminal `cancelled`. The system will set final error details such as `final_error_code=cancelled_by_user` and a summary describing the cancelled wait stage. `overall_status` will map to `failed`, so existing status filtering does not need a new top-level enum.

This distinguishes user cancellation from system failure in `current_state` and final error fields without forcing every status consumer to learn a fourth overall status bucket.

Alternative considered:

- Add `overall_status=cancelled`. Rejected because it expands the externally visible status enum for limited additional value.

### 5. Cleanup failures stop the workflow as terminal non-destructive failures

If the orchestrator cannot complete the cancellation cleanup from `cancelling`, it will not resume normal workflow progression. Instead, it will terminalize the execution through the existing pre-mutation failure path with a cancellation-specific error code such as `cancel_cleanup_failed`.

This ensures cancel requests still break the infinite-wait condition even when the compensating control-plane action itself fails.

Alternative considered:

- Retry cleanup indefinitely in `cancelling`. Rejected because it recreates the same operational problem the cancel feature is supposed to solve.

## Risks / Trade-offs

- [New persisted state and cancellation context add schema and transition complexity] → Keep the change narrowly scoped to cancellation-specific fields and document the allowed transitions clearly.
- [Operators may expect `rebooting` or `verifying` to be cancellable because they are also waits] → Return an explicit 409 conflict explaining that those states are beyond the safe-cancel boundary.
- [Cleanup actions can race with late MQ events] → Claim the execution as `cancelling` before cleanup begins so stale events are discarded by normal state checks.
- [Cancelled executions will appear under `overall_status=failed`] → Preserve the distinct `cancelled` `current_state` and `cancelled_by_user` error code so operators can distinguish intent from fault.

## Migration Plan

1. Add the new cancellation states and persisted cancellation-context field(s) to the execution model and storage schema.
2. Implement API/service cancellation claim logic and return state-aware HTTP responses.
3. Extend orchestrator action selection and recovery to own `cancelling` cleanup.
4. Update tests, API docs, and operator documentation to describe the allowed cancellation window and terminal `cancelled` state.

Rollback can remove the endpoint behavior and restore the 501 stub after reverting the new state transitions and schema changes.

## Open Questions

- None.
