## MODIFIED Requirements

### Requirement: Environment-based configuration

The daemon SHALL read all configuration from environment variables. Required variables: `API_TOKEN`, `LISTEN_ADDR`, `DB_PATH`. Optional integration variables: `SLURM_API_URL`, `SLURM_JWT_TOKEN`, `SLURM_API_USER`, `SLURM_ADMIN_USER`, `SLURM_ADMIN_JWT_TOKEN`, `OS_AUTH_URL`, `OS_PROJECT_NAME`, `OS_USERNAME`, `OS_PASSWORD`, `AMQP_URL`, `SSH_USER`, `SSH_PORT`, `SSH_OPTIONS`, `SSH_PRIVATE_KEY_PATH`, `SSH_POLL_INTERVAL`, and `SSH_POLL_TIMEOUT`.

If `SLURM_API_URL` is set, the daemon MUST treat `SLURM_API_USER` and `SLURM_JWT_TOKEN` as the default workload identity for Slurm API calls when a request or execution does not carry an explicit workload override. `SLURM_API_USER` defaults to `cloud-user` when unset. `SLURM_ADMIN_USER` defaults to `SLURM_API_USER`, and `SLURM_ADMIN_JWT_TOKEN` defaults to `SLURM_JWT_TOKEN`. Missing default workload credentials MUST NOT prevent daemon startup by themselves; instead, any request or execution that cannot resolve a complete workload identity at runtime MUST fail with a clear error before workload-scoped Slurm operations are attempted.

If any of `SSH_USER`, `SSH_PORT`, `SSH_OPTIONS`, or `SSH_PRIVATE_KEY_PATH` are set, the daemon MUST treat SSH runner configuration as enabled. When SSH runner configuration is enabled, `SSH_PRIVATE_KEY_PATH` MUST be set and reference a readable private key file. The daemon MUST construct an SSH runner from the configured SSH transport values and supply it to the orchestrator for reboot and reachability actions.

#### Scenario: Daemon starts with minimal config

- **WHEN** `API_TOKEN`, `LISTEN_ADDR`, and `DB_PATH` are set
- **THEN** daemon starts and serves the REST API

#### Scenario: Daemon starts with workload-only Slurm config

- **WHEN** `SLURM_API_URL` and `SLURM_JWT_TOKEN` are set but `SLURM_API_USER`, `SLURM_ADMIN_USER`, and `SLURM_ADMIN_JWT_TOKEN` are unset
- **THEN** daemon starts with `cloud-user` as the workload identity and reuses the workload identity for node mutation calls

#### Scenario: Daemon uses dedicated admin Slurm credentials

- **WHEN** `SLURM_ADMIN_USER` and `SLURM_ADMIN_JWT_TOKEN` are set together with workload Slurm configuration
- **THEN** daemon uses the admin identity for drain and resume calls while keeping workload identity for job submission, cancellation, and default node reads

#### Scenario: Daemon starts without a default workload token

- **WHEN** `SLURM_API_URL` is set but `SLURM_JWT_TOKEN` is not set
- **THEN** daemon still starts successfully
- **AND** only requests or executions that provide or resolve another complete workload identity may perform workload-scoped Slurm operations

#### Scenario: Request fails when no runtime workload identity can be resolved

- **WHEN** `SLURM_API_URL` is set, `SLURM_JWT_TOKEN` is unset, and a `slurm_to_openstack` request does not provide `slurm_user` and `slurm_user_token`
- **THEN** the daemon returns a clear error for that request instead of failing startup

#### Scenario: Daemon starts with SSH runner config

- **WHEN** `SSH_USER=slurm` and `SSH_PRIVATE_KEY_PATH=/run/secrets/node-key` are set and the key file is readable
- **THEN** daemon starts with an SSH runner configured for reboot and reachability actions

#### Scenario: Incomplete SSH runner config

- **WHEN** any of `SSH_USER`, `SSH_PORT`, or `SSH_OPTIONS` are set but `SSH_PRIVATE_KEY_PATH` is unset or unreadable
- **THEN** daemon exits with a clear error message indicating that `SSH_PRIVATE_KEY_PATH` is required for SSH runner configuration
