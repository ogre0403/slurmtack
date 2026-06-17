## 1. Configuration And Wiring

- [x] 1.1 Extend daemon configuration and deployment docs for `SSH_LOGIN_NODE` and `SLURM_ADMIN_TOKEN_LIFESPAN`, including the compatibility rules for `SLURM_ADMIN_JWT_TOKEN`.
- [x] 1.2 Wire startup so the Slurm client receives either the existing static admin credentials path or the new SSH-backed admin-token provider based on runtime configuration.

## 2. Admin Token Renewal Flow

- [x] 2.1 Implement an internal admin-token provider that can mint short-lived Slurm admin JWTs over SSH, cache the current token in memory, and invalidate it on auth failure.
- [x] 2.2 Refactor admin-authenticated slurmrestd operations to resolve admin credentials at request time, persist a datastore audit record for each successful SSH-minted token issuance, and retry once after token renewal only for authentication failures.

## 3. Renewal Audit Persistence

- [x] 3.1 Extend the domain/store/SQLite schema with an admin-token renewal audit record that stores timestamp, admin user, login node, and renewal trigger without storing token material.
- [x] 3.2 Add store-facing tests that verify renewal records are persisted for both initial cache fill and auth-failure-driven renewal.

## 4. Verification

- [x] 4.1 Add focused tests for config validation, SSH token issuance parsing, cache invalidation, renewal audit persistence, and single-retry admin request behavior.
- [x] 4.2 Update README and env examples to recommend `SSH_LOGIN_NODE` for long-running deployments and document the operational expectations for SSH access, `scontrol token`, and renewal audit records.
