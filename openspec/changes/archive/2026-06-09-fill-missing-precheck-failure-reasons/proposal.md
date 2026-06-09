## Why

Execution detail steps are already supposed to show operator-readable precheck failure reasons, but the live workflow only persists those reasons for some precheck branches. As a result, operators can see one failed execution with a clear blocker summary and another failed execution with only a generic failed precheck, even when the daemon already knows the concrete cause such as a missing compute service.

## What Changes

- Audit the live precheck failure paths across both switch directions and identify which branches currently fail without a persisted operator-visible reason.
- Require the orchestrator to persist a deterministic precheck failure summary whenever the daemon can identify the blocking condition, including `slurm_to_openstack` cases such as a missing or unreadable OpenStack compute service.
- Keep the existing execution step API and dashboard rendering path, but ensure failed precheck steps consistently receive the `error_summary` data those surfaces already know how to display.
- Add regression coverage for the identified missing-summary cases so future precheck branches do not silently drop the operator-visible reason.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `orchestrator`: tighten precheck failure behavior so both `openstack_to_slurm` and `slurm_to_openstack` persist operator-visible blocker summaries on failed precheck steps whenever a concrete cause is known.
- `gpu-node-switch-observability`: broaden the durable step-history requirement so failed precheck steps preserve readable rejection summaries for all supported precheck blocker types, not only the currently covered `openstack_to_slurm` readiness blockers.

## Impact

- Affected code: `internal/orchestrator`, shared step failure handling, OpenStack precheck/readiness helpers, and tests covering persisted step timelines.
- Affected operator surfaces: execution detail steps in `GET /v1/switches/:id/steps` and the dashboard execution detail view should start showing reasons for precheck failures that are currently blank.
- Dependencies: no new external systems; relies on the existing step-history store fields, API DTOs, and dashboard rendering path that already support `error_summary`.
