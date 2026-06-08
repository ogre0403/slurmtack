## Why

The archived `slurm-token-auth-exchange` change introduced session-based authentication, but the running daemon still requires `API_TOKEN` at startup and the dashboard still falls back to prompting for a static bearer token. That leaves the system in a split state where the new exchange flow exists, but deployment, middleware, and operator UX still depend on the older shared-secret model.

## What Changes

- **BREAKING** Remove `API_TOKEN` from daemon startup and runtime authentication; `/v1/*` endpoints will no longer accept the legacy static bearer token fallback.
- Align daemon configuration and startup validation with the token-exchange design so the service can start without `API_TOKEN`.
- Update API authentication middleware and server wiring to rely only on slurmtack-issued Web Session JWTs for protected endpoints.
- Update the dashboard bootstrap flow to obtain a session token through `POST /v1/auth/login` and stop prompting operators for a static API token.
- Refresh operator-facing documentation and examples so authentication setup and curl flows reflect the exchange-based model.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `daemon-deployment`: Remove `API_TOKEN` from required daemon configuration and describe startup behavior under exchange-based authentication.
- `rest-api`: Remove static bearer-token fallback and require slurmtack-issued Web Session JWTs for protected API access.
- `node-switch-dashboard`: Require dashboard bootstrap and retry flows to use the token-exchange endpoint instead of prompting for a shared API token.

## Impact

- `internal/config/config.go` and config tests
- `internal/api/middleware_auth.go`, `internal/api/server.go`, and auth-related API tests
- `docker/nginx/html/dashboard.js`
- Operator documentation such as `README.md`
