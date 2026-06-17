## ADDED Requirements

### Requirement: Persist admin token renewal audit records

The SQLite store SHALL persist one audit record for every successful Slurm admin-token issuance performed through the SSH-backed renewal path. Each record MUST include the issuance timestamp, the effective Slurm admin username, the configured login-node host, and the renewal trigger. The renewal trigger MUST distinguish at least between `cache_miss` and `auth_failure`. The store MUST NOT persist the JWT token value itself.

#### Scenario: First SSH-minted admin token is recorded

- **WHEN** the daemon successfully mints an admin token over SSH because no cached admin token exists yet
- **THEN** the store persists one renewal audit record with trigger `cache_miss`
- **AND** the record includes the issuance timestamp, admin username, and login-node host

#### Scenario: Retry-driven renewal is recorded

- **WHEN** the daemon successfully mints a replacement admin token after an admin-authenticated slurmrestd request failed authentication
- **THEN** the store persists one renewal audit record with trigger `auth_failure`
- **AND** the persisted record does not include the token value
