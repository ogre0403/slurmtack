## 1. Cancellation Cleanup Logic

- [x] 1.1 Refactor cancellation cleanup in `internal/orchestrator` so source-state rollback actions remain state-driven, while placeholder-job and lease teardown are driven by the execution's current persisted resources.
- [x] 1.2 Update the `slurm_to_openstack` cancellation path to always cancel `placeholder_job_id` when present and to delete the execution-owned lease record when present before transitioning to `cancelled`.
- [x] 1.3 Make cleanup idempotent by treating already-absent placeholder jobs or lease records as successful cleanup rather than terminal cancellation failure.

## 2. Regression Coverage

- [x] 2.1 Extend orchestrator cancellation tests to cover `awaiting_source_allocation` cancellation when both `placeholder_job_id` and a lease already exist.
- [x] 2.2 Add regression tests for retry or late-event boundaries so a cancelling execution cannot finish with a leftover lease or running placeholder job.

## 3. Verification

- [x] 3.1 Run focused Go tests for the touched cancellation, orchestrator, MQ, and store packages.
