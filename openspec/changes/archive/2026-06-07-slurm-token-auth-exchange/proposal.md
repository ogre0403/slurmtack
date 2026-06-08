## Why

Currently, slurmtack relies on a single static global `API_TOKEN` defined in `.env` for all API authentication. This approach presents several challenges in multi-user/multi-tenant environments:
- It requires sharing the highly sensitive admin token with all users who need access to the UI.
- It lacks secure audit trails, as the `RequestedBy` field is self-declared by the client without server-side verification.
- Storing long-lived Slurm JWT tokens permanently in browser `localStorage` exposes them to XSS and shared-machine credential leaks.

By introducing a credential exchange mechanism (Option A), users can authenticate using their existing Slurm JWT tokens to receive a short-lived, slurmtack-signed Web Session JWT. This removes the need for a shared static API token in the frontend, derives secure and verified identities, and mitigates credential leakage through a hybrid storage strategy (`sessionStorage` for tokens, `localStorage` for non-sensitive settings).

## What Changes

- **Authentication Endpoint**: Add a new endpoint `POST /v1/auth/login` that accepts a Slurm user and Slurm JWT token, validates them against the configured Slurm REST API, and returns a short-lived, slurmtack-signed Web Session JWT.
- **Dual-Token API Authentication**: Update the API middleware to accept either the global static `API_TOKEN` (for admin/backward compatibility) or the dynamically-signed Web Session JWT. Automatically bind the verified username from the Web Session JWT to incoming execution requests to ensure authenticated accountability.
- **Frontend Storage Refactoring**: Transition `slurm_user_token` and the retrieved `slurmtack_token` from `localStorage` to `sessionStorage` so they are automatically cleared when the tab/browser is closed, while keeping non-sensitive metadata (`slurm_account`, `placeholder_sif_file`) in `localStorage` to maintain user convenience across sessions.
- **Silent Token Auto-Renewal**: Update the UI API handler to catch `401 Unauthorized` responses and automatically attempt a silent credential exchange (using the Slurm token) to refresh the Web Session JWT and retry the failed request transparently.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `rest-api`: Added credential exchange endpoint (`POST /v1/auth/login`) and updated JWT authentication and context-binding middleware.
- `node-switch-dashboard`: Refactored frontend storage to use a hybrid `sessionStorage` / `localStorage` split and implemented silent token renewal on `401` errors.

## Impact

- **internal/api/middleware_auth.go**: Modified to support dual-token authentication (static token fallback + dynamic JWT validation).
- **internal/api/server.go**: Wired up the new `/v1/auth/login` endpoint and configured JWT signing key.
- **internal/config/config.go**: Added an option to configure a custom JWT signing key (defaulting to a randomly generated key at startup if not provided).
- **docker/nginx/html/dashboard.js**: Migrated sensitive settings storage to `sessionStorage`, implemented silent token renewal, and added handling for credential exchange during session initialization.
