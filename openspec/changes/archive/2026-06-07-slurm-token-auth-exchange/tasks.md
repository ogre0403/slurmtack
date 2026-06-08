## 1. Backend: Configuration & JWT Core Support

- [x] 1.1 Support `JWT_SIGNING_KEY` environment variable in `internal/config/config.go` and generate a secure random 32-byte key at daemon startup if empty.
- [x] 1.2 Implement dynamic JWT generation and signature verification helper functions in the API layer.

## 2. Backend: Token Verification & Exchange Endpoint

- [x] 2.1 Implement a Slurm JWT verification helper in the Slurm client that checks token validity by making a lightweight read request to the Slurm REST API.
- [x] 2.2 Add the `POST /v1/auth/login` API route and handler to validate Slurm credentials and return a slurmtack Web Session JWT.
- [x] 2.3 Write comprehensive unit tests for the token exchange endpoint covering success and rejection scenarios (expired token, mismatching username).

## 3. Backend: Dual-Token Middleware & Context Binding

- [x] 3.1 Refactor the `BearerAuth` middleware in `internal/api/middleware_auth.go` to support either the static `API_TOKEN` or dynamic Web Session JWTs.
- [x] 3.2 Bind the authenticated username to the request context inside `BearerAuth` and verify it in the handler.
- [x] 3.3 Ensure the switch creation handler reads the username from the request context and uses it to override/verify the `RequestedBy` field.
- [x] 3.4 Implement unit tests for the refactored middleware and user accountability overrides.

## 4. Frontend: Storage Strategy Refactoring

- [x] 4.1 Update `dashboard.js` to store sensitive credentials (`slurm_user_token` and `slurmtack_token`) in `sessionStorage` and other options in `localStorage`.
- [x] 4.2 Update UI initialization to correctly parse and restore settings from their respective hybrid storage locations on load.
- [x] 4.3 Update the settings save and clear functions to write/wipe both `sessionStorage` and `localStorage` cleanly.

## 5. Frontend: Silent Token Auto-Renewal

- [x] 5.1 Implement a background token exchange API call function in `dashboard.js`.
- [x] 5.2 Wrap frontend HTTP requests to intercept `401 Unauthorized` responses and initiate silent auto-renewal using `slurm_user_token` if available.
- [x] 5.3 Implement the automatic page fallback: on background exchange failure, clear session state, notify the operator of expired credentials, and open the settings drawer.

## 6. End-to-End Verification & Integration Tests

- [x] 6.1 Update `dashboard_ui_test.go` to cover the new sessionStorage/localStorage split and silent token renewal behaviors.
- [x] 6.2 Execute the full Go test suite (`go test ./...`) to ensure a compiling and clean-checking build.
