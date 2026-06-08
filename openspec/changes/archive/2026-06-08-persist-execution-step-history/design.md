## Context

The repo already has most of the pieces needed for execution step history:

- `internal/store` persists `steps` rows with sequence, status, timing, retry, exit, error, and evidence-path fields.
- `GET /v1/switches/:id/steps` already exposes those rows through the authenticated API.
- the dashboard detail drawer already calls that endpoint when an execution is selected.

The gap is in the live runtime path. `internal/engine.Runner.RunStep` can persist step records, but the deployed orchestrator mostly performs work by calling Slurm, OpenStack, SSH, MQ intake, and cancellation logic directly, then advancing state with `Runner.Transition`. That means real executions often reach the dashboard with no persisted step timeline even though structured logs show progress. The dashboard also renders only a thin status line per step, so it does not make good use of the metadata the API already returns.

## Goals / Non-Goals

**Goals:**

- Persist a durable execution timeline for the live workflow path, not just for tests or helper code that happens to use `Runner.RunStep`.
- Record both synchronous workflow actions and asynchronous wait boundaries so active executions show meaningful in-progress steps.
- Reuse the existing `steps` table and `/v1/switches/:id/steps` endpoint rather than inventing a second history model.
- Render richer step detail in the dashboard execution drawer so operators can inspect where an execution is, how long it has been there, and what failed.

**Non-Goals:**

- Adding a new read API, push stream, or websocket for live execution updates.
- Building a browser evidence explorer for stdout, stderr, or snapshot file contents.
- Replacing the current execution state machine or redefining the switch directions.

## Decisions

### 1. Add an explicit step-lifecycle helper for the live runtime path

The implementation will add a small runtime-oriented helper that owns step creation and completion against the existing store. The helper will support:

- starting a new step and allocating the next sequence number;
- reusing the latest running step when recovery re-enters the same semantic boundary;
- finishing a step as `succeeded`, `failed`, or `skipped`;
- attaching optional metadata such as host, retry count, exit code, failure class, command IDs, and evidence paths.

`Runner.RunStep` can be refactored to use the same helper internally, but the important change is that orchestrator actions, MQ wait exits, and cancellation/failure paths will no longer depend on `StepHandler` just to persist timeline history.

Alternative considered:

- Refactor every orchestrator branch into `Runner.RunStep` handlers. Rejected because long-lived waits such as allocation, drained confirmation, target-node admission, and SSH reachability do not fit a single synchronous handler call cleanly.
- Reconstruct the timeline from structured logs at API read time. Rejected because logs are not the durable source of truth for operator drilldown, and they do not guarantee one stable ordered record per execution step.

### 2. Use stable workflow step names at the action and wait-boundary level

The persisted timeline will use stable semantic step names that reflect how operators reason about a switch. The primary units are orchestrator actions and wait boundaries, for example:

- `submit_placeholder`
- `wait_for_source_allocation`
- `wait_for_target_node`
- `acquire_lease`
- `precheck`
- `quiesce_source`
- `wait_for_source_drain`
- `verify_source_quiesce`
- `reconfigure_host`
- `reboot`
- `wait_for_ssh_reachability`
- `attach_target`
- `verify_target`
- `complete_execution`
- `cancel_cleanup`

Command-backed sub-operations that already have natural names and metadata, such as `slurmd_stop`, `slurmd_disable`, `slurmd_enable`, and `slurmd_start`, may still be persisted as their own steps when they provide materially better operator detail than a single coarse action row.

This gives the UI a stable label set and avoids the current mismatch where state changes happen but no step rows exist.

Alternative considered:

- Store only raw state names as the timeline. Rejected because it tells the operator what state the execution reached, but not what action or wait boundary produced that state.
- Collapse the whole execution into one JSON blob. Rejected because it would bypass the ordered row model the API and dashboard already use.

### 3. Model waits as first-class running steps that are later closed by the event or poll path

Wait states are the main reason the current execution drawer is unhelpful in production. When the runtime enters a wait boundary, the helper will immediately persist a running step. That step stays open until the corresponding path closes it:

- MQ `allocation` closes `wait_for_source_allocation`.
- MQ `node_selected` closes `wait_for_target_node`.
- MQ `drained` or local OpenStack quiesce satisfaction closes the source-quiesce wait.
- SSH reachability success or timeout closes `wait_for_ssh_reachability`.
- user cancellation or terminal failure closes any currently running wait/action step as `skipped` or `failed`, depending on why the execution stopped progressing.

This lets active executions show a meaningful "current step" instead of an empty timeline or a drawer that only updates after completion.

Alternative considered:

- Append a new row on every poll tick or MQ progress log. Rejected because it would flood the timeline with noise instead of showing one durable step boundary with a clear start and finish.

### 4. Reuse the existing API schema and focus UI work on presentation

The `StepResponse` shape already contains the fields needed for a useful execution drawer. No new endpoint or schema is required. The dashboard will keep loading `/v1/switches/:id` and `/v1/switches/:id/steps`, but it will render the step list as richer timeline rows/cards that show:

- sequence and human-readable step label;
- current status badge;
- started/ended times when available;
- host when relevant;
- retry count when non-zero;
- exit code and error class when present;
- stdout, stderr, and snapshot paths as secondary metadata when present.

The UI will keep optional fields compact so the drawer remains readable on smaller screens, but the operator will no longer need to infer progress from a single step name and status string.

Alternative considered:

- Introduce a new dashboard-only detail endpoint that preformats the timeline. Rejected because the existing API already carries the necessary data and this change is about making the live path populate it.

## Risks / Trade-offs

- [Running waits can stay open for a long time] -> Keep `ended_at` optional, render `running` explicitly in the UI, and close the step only when the execution truly leaves that boundary.
- [Recovery or concurrent intake can create confusing duplicate steps] -> Centralize step-name constants and make the helper reuse the latest matching running step when appropriate instead of blindly appending every time.
- [Detailed step metadata can make the drawer noisy] -> Render secondary fields only when present and keep the default layout focused on name, status, and timing.
- [A code path that advances state without touching the helper would reintroduce empty histories] -> Add integration coverage across orchestrator, MQ consumer, cancellation, and failure paths instead of testing only the handler/store layer.

## Migration Plan

No schema migration or API versioning is required. The `steps` table and `GET /v1/switches/:id/steps` endpoint already exist.

Deployment plan:

1. Ship daemon changes that persist runtime step history on the live orchestrator, MQ intake, and cancellation/failure paths.
2. Ship dashboard changes that render the richer timeline metadata from the existing step response.
3. Allow existing executions and historical rows to remain readable as-is; only new or resumed executions need to produce the richer timeline.

Rollback plan:

1. Revert the daemon/dashboard code to the previous version if the timeline behavior causes operator confusion.
2. Keep the already-written step rows in place; they remain compatible with the current schema and older readers can ignore the extra populated metadata.

## Open Questions

- None. The main uncertainty is implementation detail, not product behavior.
