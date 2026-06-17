## Context

The daemon currently wires `SLURM_ADMIN_USER` and `SLURM_ADMIN_JWT_TOKEN` directly into `slurm.NewRestClient`, and the slurmrestd client stores those values as static strings for all default node reads, partition listing, default job cancellation, drain, and resume calls. That behavior works only as long as the configured admin token remains valid.

The repository already has two pieces needed for a safer design:

- deployment guidance and local docs already use `scontrol token username=... lifespan=...` for short-lived Slurm JWT generation;
- the daemon already has SSH transport configuration and a local `ssh`-based executor.

The missing piece is a runtime admin-token provider that can renew short-lived tokens without changing the public switch API or forcing operators to restart the daemon when a token expires. Because this renewal changes privileged control-plane credentials at runtime, the change also needs a durable audit trail in the datastore instead of relying only on transient logs.

## Goals / Non-Goals

**Goals:**

- Allow deployments to renew Slurm admin JWTs lazily at runtime by SSHing to a configured login node.
- Preserve backward compatibility for deployments that still provide a static `SLURM_ADMIN_JWT_TOKEN`.
- Limit renewal behavior to admin-authenticated Slurm operations so workload-scoped token behavior remains unchanged.
- Keep token refresh behavior bounded and observable by using a single retry on auth failure.

**Non-Goals:**

- Replacing the existing local `ssh` command transport with a new SSH library.
- Removing `SLURM_ADMIN_JWT_TOKEN` in this change.
- Adding proactive background refresh loops or persistent token storage.
- Changing request-scoped workload identity behavior for placeholder jobs or auth exchange flows.

## Decisions

### 1. Keep `SLURM_ADMIN_JWT_TOKEN` as a compatibility path

The change keeps static admin-token support so current deployments remain valid. When `SSH_LOGIN_NODE` is unset, behavior remains as-is: the daemon uses `SLURM_ADMIN_JWT_TOKEN` and falls back to `SLURM_JWT_TOKEN` when a dedicated admin token is not configured.

When `SSH_LOGIN_NODE` is set, SSH-based renewal becomes the primary admin-token source. `SLURM_ADMIN_JWT_TOKEN` becomes optional bootstrap state instead of a required long-lived secret. This avoids a breaking deployment change while steering new deployments toward short-lived tokens.

### 2. Introduce a dedicated login-node token issuer instead of overloading orchestration SSH behavior

The daemon already has SSH transport settings (`SSH_USER`, `SSH_PORT`, `SSH_OPTIONS`, `SSH_PRIVATE_KEY_PATH`), but the current SSH runner is semantically aimed at node operations such as reachability checks and remote `systemctl` calls. This change should add a dedicated admin-token issuer abstraction that reuses the same transport settings while taking its target host from a new `SSH_LOGIN_NODE` environment variable.

That separation keeps worker-node orchestration SSH behavior and login-node token issuance behavior distinct, even if both use the same underlying local `ssh` execution model.

### 3. Use lazy refresh with a single in-memory cached token

The client should not mint a new admin token for every request, and it should not add a background goroutine that predicts expiry. Instead, the admin-token provider keeps one in-memory token cache:

- if a cached token exists, use it;
- if no cached token exists, mint one over SSH;
- if an admin-authenticated slurmrestd request fails due to authentication, invalidate the cache, mint a fresh token, and retry the original request once.

To avoid duplicate SSH issuance when several admin requests fail authentication against the same expired token at once, renewal is keyed on the token the caller actually used: the provider only mints a replacement when the cache still holds that stale token. A concurrent caller whose cache was already rotated reuses the freshly minted token instead of minting again.

This is the lowest-complexity approach that still fixes the operational failure mode caused by expired admin tokens.

### 4. Scope runtime renewal to admin-authenticated Slurm API calls only

Only operations that currently use the default admin identity should be routed through the renewal path: default node reads, partition listing, default job cancellation, drain, and resume. Workload-scoped methods such as placeholder submission, workload-authenticated job cancellation, workload-authenticated job-state reads, and request-scoped token verification must keep using their explicit workload identities.

This keeps the new behavior narrow, makes testing simpler, and avoids silently changing semantics for user-scoped operations.

### 5. Treat only auth failures as refreshable

The retry path must be tightly scoped. HTTP `401` responses and structured Slurm API errors that clearly indicate invalid or expired authentication should trigger cache invalidation and one retry. Other failures, such as invalid node state, network errors, or Slurm-side operation errors, should return immediately without renewal.

This prevents the daemon from masking real operational failures behind unnecessary token refresh attempts.

### 6. Persist one audit record for every SSH-minted admin token issuance

Every successful admin token issuance performed through `SSH_LOGIN_NODE` should create one datastore record that captures the issuance time, the Slurm admin username, the login-node host, and the renewal trigger (`cache_miss` for first mint, `auth_failure` for retry-driven renewal). The record must not contain the JWT itself.

This keeps the renewal flow auditable across restarts and gives operators a durable source of truth that is safer than logging secrets or relying on in-memory cache state.

## Risks / Trade-offs

- [SSH access to the login node is unavailable or misconfigured] -> Fail the admin request with a clear issuance error and document that `SSH_LOGIN_NODE` requires the existing SSH transport settings to be valid.
- [Remote token-minting command differs by effective SSH user privileges] -> Standardize implementation on non-interactive command execution and document the expectation that the configured SSH identity can run `scontrol token`, either directly or via passwordless `sudo`.
- [Cached token is refreshed too aggressively or too rarely] -> Keep the policy intentionally simple: reuse until auth failure, then refresh once.
- [Renewal auditing could accidentally persist sensitive token material] -> Limit the persisted record to metadata such as timestamp, admin user, login node, and trigger, and explicitly exclude the JWT string.
- [Operators continue storing long-lived admin tokens unnecessarily] -> Preserve compatibility but update docs to recommend `SSH_LOGIN_NODE` as the preferred deployment mode.

## Migration Plan

1. Deploy a daemon build that supports `SSH_LOGIN_NODE` and `SLURM_ADMIN_TOKEN_LIFESPAN`.
2. Configure `SSH_LOGIN_NODE` together with the existing SSH transport settings and ensure the remote identity can run `scontrol token username=<SLURM_ADMIN_USER> lifespan=<seconds>`.
3. Optionally leave `SLURM_ADMIN_JWT_TOKEN` configured during rollout as bootstrap compatibility, then remove it later if the SSH renewal path proves stable.
4. Roll back by unsetting `SSH_LOGIN_NODE` and returning to the existing static `SLURM_ADMIN_JWT_TOKEN` deployment model.

## Open Questions

None.
