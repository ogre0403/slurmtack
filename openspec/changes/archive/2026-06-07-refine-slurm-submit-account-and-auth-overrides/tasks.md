## 1. Request intake and configuration

- [x] 1.1 Extend the `slurm_to_openstack` request schema, handler, and service validation to accept `slurm_account`, `slurm_user`, and `slurm_user_token`, enforce pairwise override rules, and return a client-visible error when no effective workload identity can be resolved.
- [x] 1.2 Relax config loading so missing default workload credentials no longer block daemon startup when `SLURM_API_URL` is set, while preserving the existing `SLURM_API_USER` and admin fallback behavior.

## 2. Execution persistence and orchestration wiring

- [x] 2.1 Extend `domain.Execution`, SQLite schema/migrations, store read/write paths, and execution-detail DTO mapping to persist the requested Slurm account plus the resolved workload user/token needed for async placeholder handling and to expose `requested_slurm_account` in `GET /v1/switches/:id`.
- [x] 2.2 Thread the execution-scoped workload identity through the orchestrator and related Slurm-facing paths so placeholder submission, workload-authenticated node reads, cancellation, and placeholder-agent exports all use the resolved execution profile.

## 3. Slurm client payloads and auth behavior

- [x] 3.1 Update the Slurm placeholder submit request model and `internal/slurm/restclient.go` to include `job.account` when requested and to derive `current_working_directory`, `standard_output`, and `standard_error` from `/home/<workload-user>`.
- [x] 3.2 Update Slurm client auth/header selection so workload-scoped calls use execution overrides when present, fall back to daemon defaults otherwise, and keep admin drain/resume behavior unchanged.

## 4. Verification and documentation

- [x] 4.1 Update API, service, config, and store tests to cover the new request fields, pair validation, startup fallback rules, persistence/migration behavior, and `requested_slurm_account` exposure in execution detail responses.
- [x] 4.2 Update Slurm client and orchestrator tests to assert account-aware placeholder payloads, home-directory paths, and execution-scoped workload identity selection.
- [x] 4.3 Update operator-facing docs and examples to describe the new request-body overrides, the default-env fallback behavior, and the placeholder job's account and home-directory settings.
