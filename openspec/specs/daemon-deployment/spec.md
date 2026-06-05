## ADDED Requirements

### Requirement: Multi-stage Docker build

The Dockerfile SHALL use a multi-stage build: a build stage with Go toolchain and gcc (for CGO/SQLite), and a runtime stage based on alpine with only the compiled binary and CA certificates.

#### Scenario: Build produces minimal image

- **WHEN** docker build is run
- **THEN** the final image contains only the slurmtack binary, CA certs, and a non-root user

#### Scenario: Binary is statically usable

- **WHEN** the container starts
- **THEN** the binary runs without missing shared libraries (static linking or musl-compatible)

### Requirement: Docker Compose stack

The docker-compose.yaml SHALL define at minimum three services: `nginx`, `slurmtack`, and `rabbitmq`. The services MUST communicate over a user-defined bridge network instead of `network_mode: host`. `nginx` MUST publish the stack's browser-facing HTTP port to the host and serve as the only public entrypoint for the validation page and proxied health API. `slurmtack` MUST NOT publish any host port and MUST be reachable only from the compose network, where nginx proxies `GET /api/health` to the daemon's internal `GET /health` endpoint. `rabbitmq` MUST remain available on host ports 5672 and 15672 while also participating in the compose network for service-to-service traffic.

#### Scenario: Stack starts with nginx as public entrypoint

- **WHEN** `docker-compose up` is run on the staging host
- **THEN** `nginx`, `slurmtack`, and `rabbitmq` all start successfully
- **AND** the validation page is reachable through nginx on the published HTTP port
- **AND** `GET /api/health` succeeds through nginx without requiring direct host access to slurmtack

#### Scenario: Slurmtack is not directly exposed to the host

- **WHEN** the stack is running
- **THEN** the compose definition exposes no host port mapping for the `slurmtack` service
- **AND** browser or host access must go through nginx to reach the daemon HTTP surface

#### Scenario: RabbitMQ remains externally reachable

- **WHEN** the stack is running
- **THEN** RabbitMQ management UI is accessible on port 15672 and AMQP is accessible on port 5672

### Requirement: SQLite data persistence

The docker-compose.yaml SHALL define a volume mount for the SQLite database file so that data survives container restarts.

#### Scenario: Data survives restart

- **WHEN** an execution is created via API, then `docker-compose restart slurmtack` is run
- **THEN** the execution is still queryable via GET /v1/switches/:id after restart

### Requirement: Environment-based configuration

The daemon SHALL read all configuration from environment variables. Required variables: `API_TOKEN`, `LISTEN_ADDR`, `DB_PATH`. Optional integration variables: `SLURM_API_URL`, `SLURM_JWT_TOKEN`, `SLURM_API_USER`, `SLURM_ADMIN_USER`, `SLURM_ADMIN_JWT_TOKEN`, `OS_AUTH_URL`, `OS_PROJECT_NAME`, `OS_USERNAME`, `OS_PASSWORD`, `AMQP_URL`, `SSH_USER`, `SSH_PORT`, `SSH_OPTIONS`, `SSH_PRIVATE_KEY_PATH`, `SSH_POLL_INTERVAL`, and `SSH_POLL_TIMEOUT`.

If `SLURM_API_URL` is set, the daemon MUST treat `SLURM_JWT_TOKEN` as the workload token for Slurm API calls. `SLURM_API_USER` defaults to `cloud-user` when unset. `SLURM_ADMIN_USER` defaults to `SLURM_API_USER`, and `SLURM_ADMIN_JWT_TOKEN` defaults to `SLURM_JWT_TOKEN`.

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

#### Scenario: Missing workload token

- **WHEN** `SLURM_API_URL` is set but `SLURM_JWT_TOKEN` is not set
- **THEN** daemon exits with a clear error message indicating which Slurm variable is missing

#### Scenario: Daemon starts with SSH runner config

- **WHEN** `SSH_USER=slurm` and `SSH_PRIVATE_KEY_PATH=/run/secrets/node-key` are set and the key file is readable
- **THEN** daemon starts with an SSH runner configured for reboot and reachability actions

#### Scenario: Incomplete SSH runner config

- **WHEN** any of `SSH_USER`, `SSH_PORT`, or `SSH_OPTIONS` are set but `SSH_PRIVATE_KEY_PATH` is unset or unreadable
- **THEN** daemon exits with a clear error message indicating that `SSH_PRIVATE_KEY_PATH` is required for SSH runner configuration

### Requirement: Graceful shutdown

The daemon SHALL handle SIGTERM and SIGINT by stopping acceptance of new requests, waiting for in-flight requests to complete (with a timeout), and closing the database connection cleanly.

#### Scenario: Shutdown on SIGTERM

- **WHEN** SIGTERM is sent to the daemon process
- **THEN** daemon stops accepting new connections, finishes in-flight requests within 10 seconds, closes the DB, and exits 0

### Requirement: Docker Compose environment file

The docker-compose.yaml SHALL reference an `.env` file for default configuration values, allowing operators to customize without editing the compose file.

#### Scenario: Custom token via .env

- **WHEN** operator sets `API_TOKEN=my-secret` in `.env`
- **THEN** the daemon requires that token for API access
