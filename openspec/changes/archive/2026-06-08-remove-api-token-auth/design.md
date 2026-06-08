## Context

The archived `slurm-token-auth-exchange` change already introduced the core exchange flow: `POST /v1/auth/login`, short-lived slurmtack-signed Web Session JWTs, username binding in request context, and dashboard-side token renewal. The remaining gap is that the daemon and UI still keep one foot in the legacy model:

- `internal/config/config.go` still loads and validates `API_TOKEN`.
- `internal/api/middleware_auth.go` still authorizes requests when the bearer token matches that static token.
- `internal/api/server.go` still wires the protected `/v1` group around the static token input.
- `docker/nginx/html/dashboard.js` still prompts the operator for a shared API token when no session token is present.
- `README.md` still documents `Authorization: Bearer changeme` as the default API workflow.

That split undermines the original security goal. Operators can still bypass the exchange flow with a shared secret, and deployments still appear to require `.env` state that the new design was supposed to eliminate.

## Goals / Non-Goals

**Goals:**
- Remove `API_TOKEN` from daemon startup validation and runtime authentication.
- Make protected API access consistently depend on slurmtack-issued Web Session JWTs.
- Make the dashboard bootstrap its session via `POST /v1/auth/login` instead of prompting for a legacy token.
- Update tests and operator documentation so the supported auth flow is unambiguous.

**Non-Goals:**
- Redesigning how Slurm JWTs are validated against slurmrestd.
- Changing request-scoped workload credential behavior for switch execution requests.
- Adding a new persistent session store; the existing signed JWT session model remains in place.

## Decisions

### 1. Remove static-token configuration from daemon startup

**Choice:** Eliminate `API_TOKEN` from `internal/config.Config`, stop reading it from the environment, and stop failing startup when it is absent. Keep `JWT_SIGNING_KEY` as the only auth-specific daemon setting, with the existing random-key fallback when unset.

- **Rationale:** This removes the last startup dependency on the legacy shared-secret model while preserving the stateless session design already implemented by the previous change.
- **Alternatives Considered:**
  - *Keep `API_TOKEN` as an unused backward-compatibility setting*: Rejected because it preserves operator confusion and invites future fallback logic to reappear.
  - *Make `JWT_SIGNING_KEY` mandatory*: Rejected because the current random-key fallback is operationally acceptable and already matches the archived design.

### 2. Require Web Session JWTs for all protected `/v1` routes

**Choice:** Refactor the auth middleware and server wiring so protected routes accept only valid slurmtack-issued Web Session JWTs. Remove the static-token comparison path entirely. The server startup path should still create a JWT manager independently from whether the login endpoint is enabled. The login endpoint remains conditional on having the Slurm verification dependency available.

- **Rationale:** This aligns runtime behavior with the current `rest-api` spec and with the original exchange-based trust boundary: Slurm credentials are validated only at the exchange endpoint, and all later API calls rely on a short-lived local session token.
- **Alternatives Considered:**
  - *Keep dual-mode auth for automation*: Rejected because it preserves a privileged bypass path and contradicts the request to remove `API_TOKEN` entirely.
  - *Allow raw Slurm JWTs on protected routes*: Rejected because it moves identity validation to every API call and bypasses the intended session boundary.

### 3. Replace dashboard fallback prompt with session bootstrap

**Choice:** Change dashboard startup so it attempts an immediate token exchange whenever a valid stored `slurm_user_token` is present and no current `slurmtack_token` exists. If the exchange fails, surface a clear authentication error and open the settings panel. The dashboard must never prompt for, store, or send a shared API token.

- **Rationale:** Prompting for `API_TOKEN` reintroduces the exact shared-secret workflow that the token-exchange design was meant to remove. The dashboard already has enough information to bootstrap a session safely through the exchange endpoint.
- **Alternatives Considered:**
  - *Leave manual API-token prompt as a fallback*: Rejected because it preserves a second auth path that operators will keep using.
  - *Require the operator to click a dedicated login button after reload*: Rejected because automatic bootstrap is simpler and consistent with the existing silent-renewal behavior.

### 4. Document a two-step non-dashboard API flow

**Choice:** Update operator documentation and examples so non-dashboard clients first obtain a session token from `POST /v1/auth/login`, then call protected endpoints with `Authorization: Bearer <slurmtack-token>`.

- **Rationale:** Removing `API_TOKEN` is a breaking change for curl examples and ad hoc scripts. Documentation has to show the supported replacement path explicitly.
- **Alternatives Considered:**
  - *Defer documentation cleanup to a later change*: Rejected because it would leave operators with broken examples immediately after rollout.

## Risks / Trade-offs

- **[Risk] Existing scripts that inject `API_TOKEN` will break** → *Mitigation*: Update README examples and task the implementation with replacing the legacy curl flow with the exchange-first flow.
- **[Risk] Deployments without a persistent `JWT_SIGNING_KEY` will invalidate active sessions on restart** → *Mitigation*: Keep dashboard silent renewal and document `JWT_SIGNING_KEY` as the way to preserve sessions across restarts.
- **[Risk] A daemon without Slurm verification configuration cannot mint new sessions** → *Mitigation*: Keep startup independent from that configuration, but make documentation and runtime errors explicit that `/v1/auth/login` requires the Slurm verification path.

## Migration Plan

1. Remove `API_TOKEN` from daemon config loading, validation, and protected-route wiring.
2. Update auth middleware and tests to use JWT-only authentication for protected `/v1` routes.
3. Update the dashboard bootstrap flow so it exchanges a stored Slurm token instead of prompting for a shared secret.
4. Update README and related examples to show the login-exchange flow.
5. Roll out by removing `API_TOKEN` from deployment manifests; optionally set `JWT_SIGNING_KEY` explicitly to preserve sessions across restarts.

## Open Questions

None.
