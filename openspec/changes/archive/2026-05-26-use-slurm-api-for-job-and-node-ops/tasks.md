## 1. Configuration and shared Slurm API contract

- [x] 1.1 Extend `internal/config/config.go`, daemon wiring, and environment examples to support `SLURM_API_USER`, `SLURM_ADMIN_USER`, and `SLURM_ADMIN_JWT_TOKEN` with the documented fallback rules
- [x] 1.2 Add shared request-building helpers for Slurm API base URL normalization and `X-SLURM-USER-*` header application so daemon and placeholder-agent use the same v0.0.40 contract
- [x] 1.3 Update operator-facing docs (`README.md`, env examples, and related runbooks) to show workload-token and admin-token setup for the Slurm API flow

## 2. Daemon Slurm client migration

- [x] 2.1 Update `internal/slurm/restclient.go` to target the v0.0.40 job and node endpoints, use workload headers for submit/cancel/get, and preserve structured error handling
- [x] 2.2 Implement `DrainNode` and `ResumeNode` with the documented `{"state":["DRAIN"]}` and `{"state":["RESUME"]}` payloads plus admin-credential fallback behavior and idempotent handling
- [x] 2.3 Revise `internal/slurm/restclient_test.go` to assert the new paths, headers, payload shapes, and credential-selection behavior

## 3. Placeholder-agent alignment

- [x] 3.1 Update placeholder job submission/export logic and `cmd/placeholder-agent/main.go` config parsing so the agent receives and uses the Slurm API username defaults alongside the existing token flow
- [x] 3.2 Change placeholder-agent drain polling to query the v0.0.40 node endpoint with `X-SLURM-USER-NAME` and `X-SLURM-USER-TOKEN` headers while preserving existing drained-state and exit-code behavior
- [x] 3.3 Update `cmd/placeholder-agent/main_test.go` and related integration coverage to validate the new Slurm API request contract

## 4. Focused integration validation

- [x] 4.1 Update `internal/slurm/restclient_integration_test.go` to run against the v0.0.40 contract and accept separate workload/admin credentials when available
- [x] 4.2 Run focused tests for `internal/slurm` and `cmd/placeholder-agent`, then verify in staging whether node polling succeeds with workload credentials or needs follow-up credential changes