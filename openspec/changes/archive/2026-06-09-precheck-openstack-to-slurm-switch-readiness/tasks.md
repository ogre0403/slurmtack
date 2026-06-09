## 1. Move OpenStack-to-Slurm blocker detection into precheck

- [x] 1.1 Implement a shared `openstack_to_slurm` readiness evaluation that checks compute-service visibility, resident instances, and active migrations, and produces a deterministic blocker summary.
- [x] 1.2 Update the orchestrator precheck path to use that readiness evaluation, fail `precheck_blocked` before `precheck_passed` or `source_quiescing` when blockers exist, and keep `verify_source_quiesce` as a post-quiesce backstop.

## 2. Persist and expose step rejection summaries

- [x] 2.1 Extend the step domain/store schema and write paths to persist an optional step-level `error_summary` for failed steps, including blocked precheck reasons.
- [x] 2.2 Update the execution step timeline API, DTOs, and Swagger contract so `GET /v1/switches/:id/steps` returns `error_summary` when present.

## 3. Show refusal reasons in execution detail steps

- [x] 3.1 Update the dashboard execution detail step rendering to display step `error_summary`, especially for failed `precheck_blocked` steps.
- [x] 3.2 Keep the step detail layout readable when `error_summary`, `error_class`, exit metadata, and evidence paths are all present.

## 4. Verify the behavior end to end

- [x] 4.1 Add orchestrator and OpenStack-facing tests covering resident-instance and active-migration blockers failing during precheck instead of first surfacing from `verify_source_quiesce`.
- [x] 4.2 Add store, API, and dashboard tests covering persisted `error_summary` values and their rendering in execution detail steps.
