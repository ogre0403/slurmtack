## 1. Remove daemon-level static token dependencies

- [x] 1.1 Remove `API_TOKEN` from `internal/config` loading and validation, while keeping JWT signing-key behavior aligned with the exchange-based auth design.
- [x] 1.2 Refactor API server construction and middleware so protected `/v1` routes require slurmtack-issued Web Session JWTs and no longer accept a static bearer-token fallback.

## 2. Rework auth coverage in tests

- [x] 2.1 Update middleware and server tests to create and use Web Session JWTs for authenticated requests instead of hard-coded static tokens.
- [x] 2.2 Add regression coverage proving that legacy `API_TOKEN`-style bearer values are rejected and that authenticated usernames still flow through request context correctly.

## 3. Remove dashboard fallback to shared API token

- [x] 3.1 Update `docker/nginx/html/dashboard.js` startup and retry flows to bootstrap sessions through `POST /v1/auth/login` whenever a usable stored Slurm token is available.
- [x] 3.2 Remove any prompt, storage path, or request behavior that asks for or persists a legacy shared API token, and surface clear operator-facing auth errors when token exchange cannot complete.

## 4. Align docs and examples with the exchange flow

- [x] 4.1 Update operator documentation and examples to remove `API_TOKEN` from required setup and show the exchange-first API flow (`/v1/auth/login` then protected API calls).
- [x] 4.2 Review deployment-facing examples or env documentation for stale `API_TOKEN` references and replace them with the supported Web Session JWT workflow.
