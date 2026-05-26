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

The docker-compose.yaml SHALL define at minimum two services: `slurmtack` (the daemon) and `rabbitmq`. Both services MUST use `network_mode: host`.

#### Scenario: Stack starts successfully

- **WHEN** `docker-compose up` is run on the staging host
- **THEN** both slurmtack and rabbitmq services start and slurmtack API is reachable on port 8080

#### Scenario: RabbitMQ is accessible

- **WHEN** the stack is running
- **THEN** RabbitMQ management UI is accessible on port 15672 and AMQP on port 5672

### Requirement: SQLite data persistence

The docker-compose.yaml SHALL define a volume mount for the SQLite database file so that data survives container restarts.

#### Scenario: Data survives restart

- **WHEN** an execution is created via API, then `docker-compose restart slurmtack` is run
- **THEN** the execution is still queryable via GET /v1/switches/:id after restart

### Requirement: Environment-based configuration

The daemon SHALL read all configuration from environment variables. Required variables: `API_TOKEN`, `LISTEN_ADDR`, `DB_PATH`. Optional integration variables: `SLURM_API_URL`, `SLURM_JWT_TOKEN`, `SLURM_API_USER`, `SLURM_ADMIN_USER`, `SLURM_ADMIN_JWT_TOKEN`, `OS_AUTH_URL`, `OS_PROJECT_NAME`, `OS_USERNAME`, `OS_PASSWORD`, `AMQP_URL`.

If `SLURM_API_URL` is set, the daemon MUST treat `SLURM_JWT_TOKEN` as the workload token for Slurm API calls. `SLURM_API_USER` defaults to `cloud-user` when unset. `SLURM_ADMIN_USER` defaults to `SLURM_API_USER`, and `SLURM_ADMIN_JWT_TOKEN` defaults to `SLURM_JWT_TOKEN`.

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
