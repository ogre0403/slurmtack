## Why

`GET /v1/switches/:id/steps` is already exposed to the dashboard, but in deployed runs it commonly returns an empty array because the live orchestrator path advances executions without persistently recording the underlying action and wait steps. Operators therefore lose the execution trail they need at the exact moment the UI is supposed to explain what happened.

## What Changes

- Persist a step record for each orchestrator-managed execution action and wait stage instead of relying on the limited code paths that currently call `engine.Runner.RunStep`.
- Keep the persisted execution step timeline available across normal progression, asynchronous waits, cancellation cleanup, failure handling, and completed executions so `GET /v1/switches/:id/steps` reflects real runtime history.
- Update the dashboard execution detail view to render richer step information from the existing step timeline response, including timing and failure context, so operators can inspect fine-grained progress in the UI.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `gpu-node-switch-observability`: tighten the execution-step persistence requirements so the live orchestrator path records each action or wait boundary in durable step history, not just structured logs.
- `rest-api`: strengthen the execution step timeline contract so `GET /v1/switches/:id/steps` returns the persisted runtime step history operators expect from deployed executions.
- `node-switch-dashboard`: expand the execution drilldown requirement so the detail drawer presents fine-grained step metadata instead of a thin status-only timeline.

## Impact

- Affects runtime orchestration and execution recording in `internal/orchestrator`, `internal/engine`, and related tests.
- Affects the authenticated execution detail API and response usage in `internal/api` and `docs/swagger`.
- Affects the nginx-served dashboard execution drawer in `docker/nginx/html/dashboard.js` and its UI tests/docs.
