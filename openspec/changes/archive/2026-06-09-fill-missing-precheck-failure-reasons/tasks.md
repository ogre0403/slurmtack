## 1. Precheck Failure Metadata

- [x] 1.1 Add a shared precheck-failure path in `internal/orchestrator` that can persist step `error_class` and `error_summary` before terminalizing the execution.
- [x] 1.2 Keep execution-level failure summaries aligned with the persisted precheck step reason so `FinalErrorSummary` and the failed precheck step do not diverge.

## 2. Direction-Specific Precheck Coverage

- [x] 2.1 Update `openstack_to_slurm` precheck handling to keep using deterministic blocker summaries for resident instances, active migrations, and combined blocker cases through the shared failure path.
- [x] 2.2 Update `slurm_to_openstack` precheck handling so compute-service lookup failures such as a missing `nova-compute` service persist an operator-visible step reason instead of leaving the failed precheck step blank.
- [x] 2.3 Audit remaining precheck dependency and configuration failure branches and ensure each one that terminates the execution before mutation persists a concise operator-visible reason on the failed precheck step.

## 3. Regression Coverage

- [x] 3.1 Add or update orchestrator tests covering failed precheck steps for both switch directions, asserting `error_class` and `error_summary` on the persisted step timeline as well as the execution summary.
- [x] 3.2 Add or update API and dashboard-facing regression coverage as needed to prove the existing step timeline response and UI rendering surface the newly persisted precheck reasons without additional contract changes.
- [x] 3.3 Run the relevant test suites for orchestrator, step timeline, and dashboard history/detail flows and confirm the missing-summary cases are now covered.
