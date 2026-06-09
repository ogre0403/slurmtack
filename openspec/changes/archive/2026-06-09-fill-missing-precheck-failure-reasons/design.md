## Context

The current execution detail flow already has the pieces needed to show a failed precheck reason:

- step records can persist `error_summary`;
- `GET /v1/switches/:id/steps` already returns that field when present;
- the dashboard step timeline already renders `error_summary` for failed steps.

The gap is in the live orchestrator path. `processExecution` always writes `err.Error()` into the execution-level `FinalErrorSummary`, but the shared `failStep` helper only marks the step as failed and does not persist step-level failure metadata. Today `doPrecheck` has one special `openstack_to_slurm` blocker branch that explicitly sets step `error_class` and `error_summary`, while other precheck failures such as `slurm_to_openstack` compute-service lookup failures fall back to a failed step with no readable reason in the step timeline.

This change is therefore not an API or dashboard redesign. It is a consistency fix in the precheck failure pipeline so execution-level and step-level failure data stay aligned.

## Goals / Non-Goals

**Goals:**

- Ensure every failed precheck step persists an operator-visible reason when the daemon already knows the cause.
- Cover both structured blocker summaries and generic precheck errors across `openstack_to_slurm` and `slurm_to_openstack`.
- Reuse the existing `error_summary` field and existing API/dashboard rendering path instead of introducing a new history model.
- Add explicit regression coverage for the precheck branches that currently drop the step-level reason.

**Non-Goals:**

- Redesigning the broader execution failure model for non-precheck steps such as reboot, attach, or verification.
- Changing the external step timeline schema or adding a new endpoint for failure detail.
- Storing full control-plane payloads or log blobs inside step records.

## Decisions

### 1. Centralize precheck failure metadata before the orchestrator terminalizes the execution

Precheck failure handling will stop depending on individual branches to remember to call `FinishStep(..., WithErrorSummary(...))`. Instead, the precheck path should construct one failure description object before the execution is failed. That object should carry at least:

- the failure class to persist on the step;
- the operator-visible summary for the step timeline;
- the underlying error used for logging and execution-level failure summary when needed.

This keeps step persistence and execution terminalization fed from the same source of truth. It also avoids the current split where `processExecution` always knows the failure message but `failStep` discards it.

Alternatives considered:

- Keep patching individual call sites one by one. Rejected because new precheck branches will keep regressing if the contract remains implicit.
- Make the dashboard synthesize a reason from other fields. Rejected because the missing information is lost before the API response is built.

### 2. Use deterministic summaries for structured blockers and normalized error text for generic precheck failures

Not every precheck failure is the same shape. Some failures are structured blockers, such as resident instances or active migrations, while others are single-source errors such as "compute service on <host>: service not found" or "openstack client not configured". The design will therefore support two summary sources:

- Structured blocker summaries generated from probe results in deterministic wording and order.
- Fallback summaries derived from the triggering precheck error when there is no richer blocker structure available.

This preserves the higher-quality wording already used for `openstack_to_slurm` readiness blockers while still fixing the currently blank `slurm_to_openstack` and generic-precheck cases.

Alternatives considered:

- Only persist summaries for structured blockers. Rejected because it leaves the current "compute node not installed" class of failures unresolved.
- Persist raw wrapped error strings without normalization. Rejected because nested error text can become unstable or overly noisy for operator-facing steps.

### 3. Treat the existing step timeline API and dashboard as read-only consumers of improved persisted data

No new API field is required. The durable step model, handler DTO, Swagger surface, and dashboard rendering already understand `error_summary`. Implementation work should therefore focus on making the persisted precheck step reliably contain that field, then proving the existing consumers display it end to end.

This keeps the change small and reduces rollout risk: fixing the data source should automatically improve both API responses and UI behavior.

Alternatives considered:

- Add a second dedicated `precheck_reason` field. Rejected because it duplicates the semantics of `error_summary` and would require avoidable API/UI churn.
- Change the dashboard to read execution-level `FinalErrorSummary` instead of step metadata. Rejected because the user asked for the reason inside execution steps, and step-level detail is the right contract.

### 4. Audit precheck branches by behavior, not just by direction

The implementation and tests should explicitly cover the precheck branches that can currently bypass step summaries, including:

- `openstack_to_slurm` structured readiness blockers;
- `slurm_to_openstack` compute-service lookup failures such as a missing `nova-compute` service;
- generic precheck dependency/configuration errors that still terminate before mutation.

The test matrix should assert both execution-level failure summary and step-level `error_summary`, so future refactors cannot regress one while leaving the other intact.

Alternatives considered:

- Add only the single user-reported compute-node-missing case. Rejected because the request is to comprehensively fill missing precheck reasons, not just patch one example.

## Risks / Trade-offs

- [Operator-visible summaries may expose low-level infrastructure wording] -> Normalize generic precheck errors to concise messages and keep richer diagnostics in logs/evidence.
- [A central helper can accidentally be reused for non-precheck failures with different semantics] -> Scope the helper or contract explicitly to precheck failure handling.
- [Execution-level and step-level summaries can drift if they are still built separately] -> Feed both from the same precheck failure description object and assert both in tests.
- [Some legacy executions will still have blank step reasons] -> Accept this; the change improves newly persisted executions without requiring backfill.

## Migration Plan

1. Introduce the shared precheck failure metadata flow in the orchestrator and update precheck call sites to use it.
2. Extend or adjust tests so both switch directions prove failed precheck steps persist `error_class` and `error_summary`.
3. Verify existing API and dashboard tests continue to pass with the richer step metadata.
4. Roll out as a code-only change. No schema or API version migration is required because `error_summary` already exists.

## Open Questions

- None.
