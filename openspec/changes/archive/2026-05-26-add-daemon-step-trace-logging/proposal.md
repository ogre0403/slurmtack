## Why

The switch daemon currently executes most of the orchestration path with little or no execution-scoped logging, which makes it hard to reconstruct where a run stalled, which control-plane call failed, or which state transition happened last. This needs to change now because the design already treats every switch step as observable evidence, but the current implementation only logs a few isolated events in main, MQ consumers, and dry-run mode.

## What Changes

- Require execution-scoped trace logs for every switch workflow step, including action selection, state transitions, asynchronous wait states, external control-plane calls, lease operations, SSH reachability polling, and terminal completion or failure paths.
- Standardize log fields so daemon logs always include at least execution ID, node name when known, direction, current state, target state or action, and failure classification when applicable.
- Extend the orchestration and engine layers so logs are emitted from the real workflow control path rather than only from top-level process startup or a few error branches.
- Add focused tests that verify the daemon emits step-level logs for normal progress and failure paths without changing the existing switch state machine semantics.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `gpu-node-switch-observability`: tighten observability requirements so each workflow step and state transition produces execution-scoped trace logs that operators can use alongside persisted evidence.
- `gpu-node-switch-orchestration`: require the orchestrator and engine control path to emit consistent start, wait, success, and failure logs for each daemon-managed switch action.

## Impact

Affected code includes the daemon entrypoint and workflow control path in `cmd/main.go`, `internal/orchestrator`, `internal/engine`, and any shared logging interfaces propagated into those layers. The change also affects observability-related tests and may add logger plumbing through services that currently rely on package-level logging or do not log at all.