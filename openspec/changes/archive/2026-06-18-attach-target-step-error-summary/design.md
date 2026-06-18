## Context

The execution step timeline already has the contract needed for this request:

- step records can persist `error_summary`;
- `GET /v1/switches/:id/steps` already returns that field;
- the dashboard execution drawer already renders `error_summary` for failed steps.

The gap is in the live attach path. `doAttach` starts the `attach_target` step and routes all attach failures through the shared `failStep` helper, but `failStep` only marks the step as failed and drops the underlying error text. At the same time, `processExecution` still uses `err.Error()` as the execution-level terminal summary. The result is that operators may see a failed execution with a readable final error while the failed `Attach Target` step itself lacks the detail needed for diagnosis.

## Goals / Non-Goals

**Goals:**

- Persist an operator-visible failure summary on failed `attach_target` steps in both switch directions.
- Keep the failed step summary aligned with the execution-level terminal failure summary.
- Reuse the existing step-history schema, REST response, and dashboard rendering path.
- Add regression coverage for the attach failure branches that currently drop step-level detail.

**Non-Goals:**

- Redesigning the broader failure taxonomy or terminal-state selection for attach failures.
- Adding new API fields, storage columns, or dashboard-specific response shaping.
- Persisting full logs, stack traces, or command payloads in step history.

## Decisions

### 1. Persist attach failure summaries through the shared step-failure helper

The minimal and most robust implementation is to make the shared step-failure path persist a readable summary derived from the returned error before the execution is terminalized. `doAttach` already routes target-side errors, missing dependency errors, and readiness-check errors through `failStep`, so fixing that shared helper closes the attach gap without duplicating logic in each branch.

Alternative considered: special-case `doAttach` and write `WithErrorSummary(...)` at every attach failure call site.
This was rejected because the bug is not unique to one attach branch; it comes from the shared step-closing helper dropping the error text.

### 2. Use the same underlying error text for step and execution summaries

`processExecution` already terminalizes failed executions with `err.Error()`. The step timeline should use the same underlying summary source for attach failures so operators do not have to reconcile one message on the execution and another on the failed step. The summary should stay concise and operator-readable rather than introducing a second attach-specific formatting layer.

Alternative considered: synthesize a new attach-only summary string for the step timeline.
This was rejected because it creates unnecessary divergence between two operator surfaces that describe the same failure.

### 3. Keep the contract change in observability, not in API shape

No REST or dashboard contract change is required. The API already serializes `error_summary`, and the dashboard already renders it for failed steps. This change is therefore a persistence and consistency fix in the live orchestrator path, not a new read-surface feature.

Alternative considered: add a new step field or make the dashboard fall back to execution-level errors.
This was rejected because the missing information should be preserved on the failed step record itself, which is the correct durable source of truth.

### 4. Prove both attach directions end to end with step-history assertions

Regression coverage should exercise the actual attach action entry points:

- `slurm_to_openstack` failures while enabling the OpenStack compute service;
- `openstack_to_slurm` failures while restoring Slurm attachment readiness.

Each test should assert both the execution-level terminal summary and the persisted `attach_target` step `error_summary`, so future refactors cannot fix one surface while regressing the other.

Alternative considered: add only a helper-level unit test for `failStep`.
This was rejected because the user-visible bug appears through real attach orchestration behavior, not just the helper in isolation.

## Risks / Trade-offs

- [Shared failed-step persistence may populate `error_summary` for more than just attach failures] -> Accept this as a safe consistency improvement and keep regression assertions focused on the attach contract introduced by this change.
- [Raw error text can be noisy for operators] -> Reuse the existing concise failure text already chosen for execution terminal summaries and avoid storing verbose logs in step records.
- [This change overlaps the active attach-terminalization fix] -> Land alongside or after `fix-attach-failure-terminal-state` so attach failures both terminalize correctly and preserve step-level detail.

## Migration Plan

No schema or API migration is required.

1. Update the shared failed-step persistence path used by `doAttach`.
2. Add attach-path regression tests for both switch directions.
3. Deploy with or after the attach terminal-state fix so operators see both the terminal failure and the failed step summary on the same execution.

Rollback is code-only: revert the orchestrator/test changes and continue reading existing step rows as-is.

## Open Questions

- None.
