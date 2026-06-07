## Context

The current `slurm_to_openstack` path only persists `slurm_constraint` and `slurm_partition`, while `internal/slurm/restclient.go` builds placeholder job submissions from daemon-wide `SLURM_API_USER` / `SLURM_JWT_TOKEN` defaults and `/tmp`-based working, stdout, and stderr paths. `internal/config/config.go` also rejects daemon startup when `SLURM_API_URL` is set but `SLURM_JWT_TOKEN` is unset, which blocks the request-scoped workload credentials the new API contract needs.

This change is cross-cutting because the request-scoped Slurm workload identity must survive beyond the HTTP request into the async orchestrator, placeholder-agent environment export, job cancellation, and restart recovery. The design also has to preserve the separate admin credential path used for drain/resume and avoid leaking workload credentials through read APIs or logs.

## Goals / Non-Goals

**Goals:**

- Support `slurm_to_openstack` request fields `slurm_account`, `slurm_user`, and `slurm_user_token`
- Resolve an effective workload identity at request time using request override first and daemon defaults second, with clear request-time rejection when no complete identity exists
- Persist the execution-scoped Slurm submit settings needed by the async placeholder lifecycle and restart recovery
- Submit placeholder jobs with `job.account` plus `/home/<effective-user>` working, stdout, and stderr paths
- Keep workload credentials out of read APIs and structured logs while exposing the non-secret requested Slurm account in execution detail metadata

**Non-Goals:**

- Changing `openstack_to_slurm` request semantics or the admin credential routing model
- Introducing token minting, refresh, encryption at rest, or an external secret store
- Making placeholder-job home-directory resolution configurable beyond the documented `/home/<user>` convention
- Exposing request-scoped workload credentials through GET endpoints or dashboard read models

## Decisions

### Resolve and persist an effective Slurm submit profile when the request is admitted

**Choice:** For `slurm_to_openstack`, the API/service layer will resolve one effective workload identity before the execution is created:

1. Use `slurm_user` plus `slurm_user_token` from the request when both are present.
2. Otherwise fall back to the daemon's configured workload identity from `SLURM_API_USER` / `SLURM_JWT_TOKEN`, including the existing `cloud-user` default for `SLURM_API_USER`.
3. If neither source produces a complete identity, reject the request before persisting the execution.

The execution record will be extended with the non-secret requested Slurm account plus the resolved workload user/token so the orchestrator, cancel path, and restart recovery all use the same identity.

**Rationale:** Placeholder submission, later job cancellation, and placeholder-agent env export happen asynchronously after the HTTP request has completed. Recomputing identity later from the environment would make request overrides non-durable and break restart behavior.

**Alternatives considered:**

- Re-read env defaults later and use request overrides only for the initial submit: rejected because later workload operations could diverge from the identity that created the placeholder job
- Keep overrides only in memory keyed by execution ID: rejected because process restarts would strand in-flight executions without their workload credentials

### Expose request-scoped workload fields only on `slurm_to_openstack`, and validate them as a pair

**Choice:** `POST /v1/switches` will accept three new `slurm_to_openstack` fields: `slurm_account`, `slurm_user`, and `slurm_user_token`. `slurm_account` is optional. `slurm_user` and `slurm_user_token` form an all-or-nothing override pair, so providing only one is a client error.

**Rationale:** Pairwise validation prevents ambiguous mixed-source identities such as request user plus env token. Keeping these fields scoped to `slurm_to_openstack` matches the only flow that creates placeholder jobs today.

**Alternatives considered:**

- Use partial fallback for a half-specified override: rejected because it obscures which identity was actually used
- Reuse env variable names (`slurm_api_user`, `slurm_jwt_token`) in the request body: rejected because the API contract should describe the Slurm user/token being supplied, not the daemon's internal env variable names

### Build placeholder submit payloads from the effective workload user

**Choice:** When submitting the placeholder job, the Slurm client will:

- add `job.account` when `slurm_account` is set for the execution;
- set `current_working_directory` to `/home/<effective-user>`;
- set `standard_output` and `standard_error` to files under `/home/<effective-user>/`;
- export `SLURM_API_USER` and `SLURM_JWT_TOKEN` matching the execution's effective workload identity into the placeholder script.

**Rationale:** The documented target Slurm environment already uses `/home/${API_USER}` and account-based job submission. Matching that contract removes the current `/tmp` assumption and makes the placeholder agent see the same identity that submitted the job.

**Alternatives considered:**

- Keep `/tmp` for stdout/stderr and only add `account`: rejected because the target environment already expects user-home paths, and changing only part of the payload leaves the contract inconsistent
- Discover the home directory dynamically from local OS state or Slurm APIs: rejected for now because the target deployment convention is already documented and a generalized lookup adds unnecessary complexity

### Keep workload overrides separate from admin mutation credentials

**Choice:** Request-scoped workload overrides will affect workload-scoped Slurm actions for that execution: placeholder submission, workload-authenticated node reads, job cancellation, and placeholder-agent exports. Drain and resume will continue to use `SLURM_ADMIN_USER` / `SLURM_ADMIN_JWT_TOKEN` with the existing fallback to workload credentials when dedicated admin values are absent.

**Rationale:** The user request is specifically about overriding `SLURM_API_USER` and `SLURM_JWT_TOKEN`. Reusing those request-scoped credentials for admin mutations would silently widen their privilege requirements and change an unrelated contract.

**Alternatives considered:**

- Use request-scoped credentials for every Slurm call in the execution: rejected because it couples placeholder-job workload credentials to admin mutation permissions

### Expose `slurm_account` as read-safe operator metadata while keeping credentials internal-only

**Choice:** The implementation will surface the requested `slurm_account` through the execution detail API alongside the existing requested Slurm constraint/partition metadata. The resolved workload user/token will stay out of API DTOs, dashboard read models, and structured logs. SQLite becomes a secret-bearing store for this change, and operator documentation should call that out explicitly.

**Rationale:** Operators need to confirm which Slurm account a placeholder job was charged against, and `slurm_account` is not secret material. By contrast, exposing workload user/token through normal read paths would create avoidable credential leakage.

**Alternatives considered:**

- Keep `slurm_account` internal-only with the credentials: rejected because it hides useful execution metadata that operators need for diagnosis and auditing
- Avoid persistence and require env fallback after restart: rejected because it makes request-scoped overrides unreliable
- Introduce a separate external secret manager in this change: rejected because it is outside the current architecture and user request

## Risks / Trade-offs

- **SQLite will now store workload tokens for accepted `slurm_to_openstack` executions** -> Mitigation: keep the fields internal-only, avoid log and response exposure, and document that the database must be treated as sensitive
- **A daemon can start without default workload credentials, so some requests will now fail later at request time instead of startup** -> Mitigation: validate `slurm_to_openstack` requests before execution creation and return explicit client-facing errors when no effective identity can be resolved
- **`/home/<user>` may not match every cluster's home-directory layout** -> Mitigation: align to the documented target environment now and treat configurable home roots as a follow-up if another deployment needs them
- **Request-supplied workload tokens may expire before a long-running execution finishes** -> Mitigation: surface Slurm authentication failures clearly and rely on operator-chosen token lifetimes; token refresh remains out of scope

## Migration Plan

1. Add additive execution persistence for the requested Slurm account and resolved workload identity so existing databases migrate forward without destructive changes.
2. Relax startup validation so missing default workload credentials no longer block the daemon when `SLURM_API_URL` is configured.
3. Update request validation, orchestrator/slurm client wiring, and placeholder job submission payload generation to use the execution's effective workload profile.
4. Revise unit tests, API tests, store migration tests, and Slurm client tests for the new request fields, fallback rules, and submit payload.
5. Update operator docs and examples to show account-aware placeholder jobs and the request-body credential override path.

Rollback is straightforward because the schema changes are additive. Reverting the code leaves the extra columns unused while restoring the previous env-only submit behavior.

## Open Questions

- None.
