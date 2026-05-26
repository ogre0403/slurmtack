## Context

The current Slurm integration is split across two implementations with different assumptions. `internal/slurm/restclient.go` hard-codes slurmrestd v0.0.38, uses `Authorization: Bearer`, and leaves node drain/resume unimplemented. `cmd/placeholder-agent/main.go` separately polls node state through slurmrestd, but it also hard-codes v0.0.38 and Bearer auth. The target environment documentation shows a different contract: slurmrestd v0.0.40, `X-SLURM-USER-NAME` plus `X-SLURM-USER-TOKEN` headers, and working `POST /node/{name}` support for `DRAIN` and `RESUME`.

The change is cross-cutting because it affects daemon configuration, the shared Slurm client, placeholder-agent polling, tests, and operator runbooks. A minimal implementation also has to account for the privilege split in the target environment, where normal job actions can use a workload token while node mutations may require an admin or operator token.

## Goals / Non-Goals

**Goals:**

- Standardize daemon and placeholder-agent Slurm HTTP calls on the deployed slurmrestd v0.0.40 contract
- Keep the existing `slurm.Client` interface stable while implementing real API-backed drain/resume behavior
- Support elevated credentials for node mutations without forcing a breaking configuration change for existing workload-token setups
- Preserve the current event-driven flow in which the placeholder agent publishes allocation and drained events
- Add focused tests and documentation that reflect the actual request paths, headers, and payloads used in staging

**Non-Goals:**

- Redesign the orchestration state machine, MQ topology, or placeholder-agent event model
- Implement token minting, refresh, or secret distribution workflows
- Support multiple slurmrestd versions at runtime beyond the deployed v0.0.40 contract
- Replace placeholder-agent polling with daemon-side drain verification in this change

## Decisions

### Normalize all Slurm API requests on the v0.0.40 endpoint family

**Choice:** Treat slurmrestd v0.0.40 as the supported contract for job submit, job cancel/query, node lookup, drain, and resume. Both daemon and placeholder-agent code paths will build requests against the v0.0.40 base and stop hard-coding v0.0.38.

**Rationale:** The target environment already documents and exposes v0.0.40 endpoints for the operations this system needs. Leaving one path on v0.0.38 while another path moves to v0.0.40 would keep the contract inconsistent and make tests misleading.

**Alternatives considered:**

- Keep v0.0.38 in code and rely on manual `scontrol` for node mutations: rejected because it does not match the deployed API behavior and keeps the core gap unfixed
- Make the API version fully operator-configurable: rejected for now because the environment contract is known and version negotiation would add complexity with little value

### Switch request authentication to Slurm header-based identity selection

**Choice:** Replace Bearer-only authentication with `X-SLURM-USER-NAME` and `X-SLURM-USER-TOKEN` headers everywhere the system talks to slurmrestd.

The daemon will use two credential profiles:

- primary credentials: `SLURM_API_USER` plus `SLURM_JWT_TOKEN`
- admin credentials: `SLURM_ADMIN_USER` plus `SLURM_ADMIN_JWT_TOKEN`

Fallback rules:

- `SLURM_API_USER` defaults to `cloud-user` when unset, matching the current hard-coded behavior
- `SLURM_ADMIN_USER` defaults to `SLURM_API_USER`
- `SLURM_ADMIN_JWT_TOKEN` defaults to `SLURM_JWT_TOKEN`

Operation mapping:

- submit/cancel placeholder jobs: primary credentials
- drain/resume node: admin credentials after fallback resolution
- daemon-side node reads: primary credentials
- placeholder-agent drain polling: primary credentials

**Rationale:** This matches the documented Slurm API usage while keeping existing `SLURM_JWT_TOKEN`-only environments functional. It also lets operators supply stronger credentials only where node mutation needs them.

**Alternatives considered:**

- Require admin credentials for all Slurm operations: rejected because it increases privilege exposure and would force an unnecessary configuration break
- Add a third dedicated read-only node credential profile immediately: deferred because the current system only proves a need for workload and mutation scopes

### Implement node mutation payloads exactly as the deployed API expects

**Choice:** Encode node updates as `POST /slurm/v0.0.40/node/{name}` with payloads shaped like the documented examples: `{"state":["DRAIN"],"reason":"..."}` for drain and `{"state":["RESUME"]}` for resume.

The client will continue to surface structured API errors, and it will keep drain/resume idempotent by treating already-drained or already-resumed responses as success when Slurm returns a known semantic error.

**Rationale:** The deployed API examples are more specific than the old tests and align with the stated environment. Matching the actual payload shape avoids another mismatch where the code sends a syntactically valid but operationally unsupported request.

**Alternatives considered:**

- Keep lowercase string payloads from the original tests: rejected because they do not match the documented v0.0.40 examples
- Shell out to `scontrol` for drain/resume while using HTTP for reads: rejected because it creates mixed control paths and weakens observability

### Keep placeholder-agent polling, but align it with the shared contract

**Choice:** Keep the current placeholder-agent lifecycle intact and only change its Slurm HTTP contract: use the normalized v0.0.40 path builder, send Slurm identity headers, and keep polling `GET /node/{hostname}` until the node is drained.

**Rationale:** This preserves the current asynchronous event flow and avoids widening the change into engine or MQ redesign. The agent only needs the same request contract correction already required by the daemon.

**Alternatives considered:**

- Move drain confirmation back to the daemon: rejected for this change because it would ripple into orchestration timing and MQ event responsibilities

## Risks / Trade-offs

- **Node reads may require elevated privileges in some environments** -> Mitigation: default drain/resume to admin credentials, keep placeholder-agent on primary credentials first, and verify staging behavior with focused integration coverage before rollout
- **Deprecated v0.0.40 endpoints may disappear in a future upstream Slurm upgrade** -> Mitigation: document that this change targets the currently deployed environment and keep integration tests pinned to the live contract
- **Operators may already store `SLURM_API_URL` with or without the version suffix** -> Mitigation: normalize the configured base URL so both host-root and already-versioned inputs resolve to the same v0.0.40 request paths
- **Using primary credentials inside placeholder jobs still exposes a Slurm token on compute nodes** -> Mitigation: keep the token scope as low as the environment allows and prefer separate admin credentials only for daemon-side mutation calls

## Migration Plan

1. Update the Slurm client and placeholder-agent request builders to use the normalized v0.0.40 path and Slurm identity headers.
2. Add optional `SLURM_API_USER`, `SLURM_ADMIN_USER`, and `SLURM_ADMIN_JWT_TOKEN` configuration handling with fallback to the existing `SLURM_JWT_TOKEN`-based setup.
3. Revise unit and integration tests to assert the new paths, headers, and node mutation payloads.
4. Update `.env` examples and runbooks to document how to mint workload and admin Slurm JWTs.
5. Roll out first in staging with both workload and admin credentials configured; if node polling fails under workload credentials, decide whether to elevate the primary credential set or schedule a follow-up for dedicated read credentials.

Rollback is low risk because the change does not alter persistent data formats. Reverting to the previous release restores the prior Slurm client behavior and configuration handling.

## Open Questions

- Does `GET /slurm/v0.0.40/node/{name}` in the target environment succeed with the workload credential set, or does placeholder-agent polling require a more privileged token?
- Does job cancellation on v0.0.40 preserve the same endpoint and response envelope assumed by the existing `CancelJob` implementation, or is a focused compatibility adjustment required during implementation?