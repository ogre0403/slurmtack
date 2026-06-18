## Why

When `Attach Target` fails after the host becomes reachable, operators can see the execution-level failure but the failed execution step may still lack the error detail needed to judge what actually went wrong. The step timeline and dashboard already support `error_summary`, so this gap should be closed where the attach failure is persisted.

## What Changes

- Persist an operator-visible error summary on failed `attach_target` steps when target attachment fails in either switch direction.
- Keep the failed step summary aligned with the execution's terminal failure summary so operators do not need to compare conflicting messages.
- Add regression coverage for attach failures that currently fail the execution without preserving step-level error detail.
- Keep the existing `GET /v1/switches/:id/steps` response shape and dashboard rendering path unchanged.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `gpu-node-switch-observability`: require failed `attach_target` steps to preserve operator-visible error summaries when target attachment terminates the execution.

## Impact

- Affected code: shared step-failure handling in `internal/orchestrator`, attach action paths, and step-history tests.
- Affected operator surfaces: `GET /v1/switches/:id/steps` and the dashboard execution detail drawer should show readable attach failure reasons without API changes.
- Dependencies: no new external systems or schema changes; reuses the existing step `error_summary` field and dashboard rendering support.
