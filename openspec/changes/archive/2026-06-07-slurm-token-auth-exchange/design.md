## Context

The slurmtack daemon protects its REST API via a static token comparison against `API_TOKEN`. The dashboard frontend (Nginx client) persists this static token, along with the user's `slurm_user_token` (a long-lived Slurm JWT) and `slurm_account` in browser `localStorage`.
While functional, this model has security drawbacks:
- Sharing the static global `API_TOKEN` exposes admin privileges to standard users.
- Storing high-privilege Slurm JWTs in `localStorage` persists them permanently on disk, presenting high XSS and shared-terminal leakage risks.
- Lack of server-side validation of user identity prevents accurate audit logging.

By implementing an exchange model where a user presents their Slurm JWT, slurmtack validates it against the Slurm control plane, and issues a short-lived, locally-signed Web Session JWT, we can isolate API access, authenticate users individually, and clear sensitive tokens from disk on tab close using a hybrid `sessionStorage` approach.

## Goals / Non-Goals

**Goals:**
- Provide a token exchange endpoint `POST /v1/auth/login` to trade a valid Slurm user + Slurm JWT for a short-lived slurmtack Web Session JWT.
- Automatically validate user/token authenticity via a lightweight read request to the Slurm REST API.
- Support dual-token authentication in the `BearerAuth` middleware, fallback-supporting the static `API_TOKEN`.
- Store sensitive tokens (`slurm_user_token` and `slurmtack_token`) in browser `sessionStorage` (cleared on tab close).
- Preserve user settings convenience by storing non-sensitive parameters (`slurm_account`, `placeholder_sif_file`) in `localStorage`.
- Implement background silent token renewal on `401 Unauthorized` responses in the frontend.

**Non-Goals:**
- Creating a persistence database of sessions/tokens on the backend; the dynamic web session tokens will be completely stateless signed JWTs.
- Building standard MFA/2FA (like TOTP or SMS) directly within slurmtack.
- Re-architecting non-workload admin commands (drain/resume) to require request-scoped user authorization; admin commands remain bound to the configured daemon-level admin identity.

## Decisions

### 1. Stateless, locally-signed JWTs for dynamic Web Sessions

**Choice**: Use stateless, signed JSON Web Tokens (JWT) to represent authenticated web sessions. 
- The JWT contains a `sub` claim representing the verified Slurm username, an `iat` (issued at) claim, and an `exp` (expires at) claim set to 1 hour from issuance.
- The JWT is signed using HS256 with a key loaded from the `JWT_SIGNING_KEY` environment variable. If the environment variable is empty, slurmtack generates a cryptographically secure random 32-byte key on startup.
- **Rationale**: Keeps slurmtack stateless and lightweight without requiring a database table or Redis cache for session tracking. Generating a random key on startup is safe; any daemon restart simply invalidates active web sessions, which will be seamlessly and silently re-authenticated by the frontend using the saved Slurm token.
- **Alternatives Considered**:
  - *SQLite sessions table*: Rejected because database writes on every login create needless contention, and session cleanup workers would be required.
  - *Proxying Slurm JWT validation on every API call*: Rejected due to high latency and the risk of rate-limiting/overloading the Slurm REST API.

### 2. Validation of Slurm JWTs via Slurm REST API

**Choice**: Validate the presented Slurm JWT by making a lightweight, read-only GET request to the Slurm REST API (e.g. `/slurm/v0.0.40/partitions`) with the user credentials as headers.
- Before verifying, the slurmtack server decodes the JWT locally (without verifying its signature) to ensure its `sub` (or equivalent username claim) matches the requested username. This prevents a user from providing a valid token belonging to someone else.
- **Rationale**: Since slurmtack does not possess the Slurm cluster's private JWT signing key, proxying a fast request to the Slurm REST API is a robust, lightweight, and standard method to verify both token authenticity and active network viability.
- **Alternatives Considered**:
  - *Local signature verification*: Rejected because slurmtack does not (and should not) have access to the Slurm cluster's private signature key or shared secret.

### 3. Dual-Token Authentication Middleware and User Context Binding

**Choice**: Refactor `BearerAuth` middleware to support both authentication strategies:
- Static Fallback: If the bearer token matches the configured `API_TOKEN`, the request is authorized under the `admin` (or `cloud-user`) identity.
- Signed Web JWT: If the bearer token is a valid, locally-signed Web JWT, the request is authorized under the username parsed from the `sub` claim.
- The authenticated username is set in the Gin context (e.g. `c.Set("username", username)`). Request handlers will use this context to populate `RequestedBy` automatically, ensuring auditability.
- **Rationale**: Keeps backward compatibility for automation/CLI pipelines that depend on the static `API_TOKEN`, while establishing a secure, verified username context for the UI.

### 4. Hybrid Storage Strategy and Silent Renewal in Nginx Frontend

**Choice**:
- Split storage responsibilities in `dashboard.js`:
  - **`sessionStorage`**: Stores sensitive tokens (`slurm_user_token` and `slurmtack_token`). Persistent across page refreshes (F5), but securely wiped when the browser tab/window is closed.
  - **`localStorage`**: Stores non-sensitive defaults (`slurm_account` and `placeholder_sif_file`).
- Implement an HTTP fetch client wrapper in `dashboard.js`. If an API call fails with `401 Unauthorized`:
  - Check if `slurm_user_token` exists in `sessionStorage`.
  - If present, perform a silent `POST /v1/auth/login` in the background.
  - On success, store the new Web JWT (`slurmtack_token`) in `sessionStorage` and retry the original API request.
  - On failure (e.g., Slurm token expired), clear session tokens, show an explicit error, and open the settings panel to prompt for a new Slurm token.
- **Rationale**: Safeguards high-value Slurm JWTs from being written permanently to disk, shielding them from extraction via stolen laptops or shared endpoints, while preserving a completely seamless refresh and session persistence experience.

## Risks / Trade-offs

- **[Risk] Daemon restarts invalidate active Web JWTs** → *Mitigation*: The frontend's silent token renewal automatically recovers. When a request returns a `401` after a restart, a background credential exchange seamlessly obtains a new Web JWT without the user ever noticing.
- **[Risk] High-privilege Slurm JWTs exposed to XSS during active sessions** → *Mitigation*: While `sessionStorage` is still accessible by scripts in the same origin, it is vastly more secure than `localStorage` because it is never written to disk, has a short lifespan, and is bound strictly to the active tab.
- **[Risk] Token validation adds login latency** → *Mitigation*: The validation request is only sent to Slurm REST API during login or once-per-hour renewal. Subsequent API requests are verified locally and statelessly via HMAC, which takes less than a millisecond.
