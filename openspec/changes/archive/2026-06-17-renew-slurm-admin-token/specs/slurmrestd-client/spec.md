## ADDED Requirements

### Requirement: Resolve admin-authenticated Slurm requests with renewable credentials

The client SHALL execute default node reads, partition listing, default job cancellation, drain, and resume through one effective admin identity. When `SSH_LOGIN_NODE` is unset, the effective admin identity MUST come from the configured static admin credentials with the existing fallback to the workload identity. When `SSH_LOGIN_NODE` is set, the client MUST obtain a short-lived admin token for `SLURM_ADMIN_USER` over the configured SSH login-node path before issuing an admin-authenticated request, MUST cache the current minted token in memory for reuse across later admin-authenticated requests, and MAY seed that cache from `SLURM_ADMIN_JWT_TOKEN` when one is configured.

Every successful admin token issuance performed over `SSH_LOGIN_NODE` MUST create one datastore audit record containing the issuance timestamp, the effective `SLURM_ADMIN_USER`, the configured `SSH_LOGIN_NODE`, and the renewal trigger. The renewal trigger MUST distinguish at least between an initial cache fill and a retry after admin-authentication failure. The datastore audit record MUST NOT contain the minted JWT value.

If an admin-authenticated request fails because the admin token is invalid or expired, the client MUST invalidate the cached admin token, mint a fresh token once, persist the renewal audit record, and retry the original admin-authenticated request once. If the retried request still fails for authentication reasons, the client MUST return that error. The client MUST NOT mint a new token or retry when the failure is unrelated to authentication.

#### Scenario: Static admin token is used when SSH renewal is disabled

- **WHEN** the client performs `DrainNode` or `ListPartitions` and `SSH_LOGIN_NODE` is unset
- **THEN** it sends the configured static admin identity headers to slurmrestd
- **AND** it does not attempt SSH-based token issuance

#### Scenario: First admin request mints a token over SSH

- **WHEN** the client performs an admin-authenticated Slurm request and `SSH_LOGIN_NODE=login-01` is set
- **AND** no cached admin token exists yet
- **THEN** the client obtains a short-lived token for `SLURM_ADMIN_USER` through the configured SSH login-node path
- **AND** it stores one datastore audit record for that issuance with trigger `cache_miss`
- **AND** it uses that minted token for the outgoing slurmrestd request

#### Scenario: Authentication failure refreshes the cached admin token once

- **WHEN** an admin-authenticated Slurm request receives an authentication failure from slurmrestd
- **AND** `SSH_LOGIN_NODE` is set
- **THEN** the client invalidates its cached admin token
- **AND** it mints one fresh admin token over SSH
- **AND** it stores one datastore audit record for that issuance with trigger `auth_failure`
- **AND** it retries the original admin-authenticated request once with the renewed token

#### Scenario: Repeated authentication failure returns an error

- **WHEN** an admin-authenticated Slurm request receives an authentication failure
- **AND** the single retry with a renewed token also receives an authentication failure
- **THEN** the client returns the Slurm error to the caller
- **AND** it does not continue retrying

#### Scenario: Non-authentication failure is not retried

- **WHEN** an admin-authenticated Slurm request fails due to an unrelated Slurm error such as invalid node state or a rejected operation
- **THEN** the client returns that error immediately
- **AND** it does not mint a new admin token for that failure
