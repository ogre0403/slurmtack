## Why

Switch executions can currently remain stuck in long-running or unbounded wait states such as waiting for MQ correlation, placeholder allocation, or source-quiesce polling, but `/v1/switches/:id/cancel` is still a stub. Operators need a controlled way to stop waiting without pretending that every in-flight state is safe to abort.

## What Changes

- Implement `POST /v1/switches/:id/cancel` for both `slurm_to_openstack` and `openstack_to_slurm`.
- Add an explicit cancellation lifecycle for approved wait states only, instead of allowing cancellation from arbitrary mutation states.
- Define the cleanup actions required to cancel safely while the execution is still waiting, including releasing leases and undoing wait-stage control-plane holds when needed.
- Keep post-detach and post-reboot execution states non-cancellable so operator cancellation does not masquerade as a safe rollback after deeper host mutation has begun.

## Capabilities

### New Capabilities

- `switch-cancellation`: operator-driven cancellation of executions that are parked in approved wait states, including cancellation claim, direction-specific cleanup, and terminal cancellation outcome.

### Modified Capabilities

- `rest-api`: replace the `/v1/switches/:id/cancel` 501 stub with a state-aware cancel endpoint and client-visible conflict/not-found responses.
- `orchestrator`: treat cancellation-claimed executions as no longer eligible for normal wait progression and recover unfinished cancellation cleanup after restart.

## Impact

- Affects `internal/api`, `internal/service`, `internal/orchestrator`, `internal/domain`, and transition/runner logic for cancellation state handling.
- Likely affects persistence and status reporting so cancelled executions are distinguishable from completed workflows and ordinary failures.
- Uses existing Slurm and OpenStack client capabilities for cleanup, especially placeholder job cancellation, node resume, compute-service enablement, and lease release.
