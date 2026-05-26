## MODIFIED Requirements

### Requirement: Environment variable configuration

The agent SHALL read all configuration from environment variables. It MUST exit with code 1 if any required variable (`EXECUTION_ID`, `AMQP_URL`, `SLURM_API_URL`, `SLURM_JWT_TOKEN`) is missing or empty. `SLURM_API_USER` is optional and defaults to `cloud-user` when unset.

#### Scenario: All required vars present

- **WHEN** agent starts with all required environment variables set
- **THEN** agent proceeds with startup

#### Scenario: Missing EXECUTION_ID

- **WHEN** agent starts without EXECUTION_ID set
- **THEN** agent prints error to stderr and exits with code 1

#### Scenario: Missing AMQP_URL

- **WHEN** agent starts without AMQP_URL set
- **THEN** agent prints error to stderr and exits with code 1

#### Scenario: Missing SLURM_JWT_TOKEN

- **WHEN** agent starts without SLURM_JWT_TOKEN set
- **THEN** agent prints error to stderr and exits with code 1

### Requirement: Poll slurmrestd for drain state

The agent SHALL poll `GET /slurm/v0.0.40/node/{hostname}` at the configured interval (default 5s) to check if the node has reached a drained state. Each poll request MUST include `X-SLURM-USER-NAME` and `X-SLURM-USER-TOKEN` headers derived from the configured workload Slurm identity. Drained states are: `drained`, `drained*`, `down`, `down*`.

#### Scenario: Node drains within timeout

- **WHEN** slurmrestd reports node state as `drained` during polling
- **THEN** agent proceeds to publish node_drained event

#### Scenario: Node state is `drained*`

- **WHEN** slurmrestd reports node state as `drained*` (drain pending jobs complete)
- **THEN** agent considers this as drained and proceeds

#### Scenario: Poll request uses Slurm identity headers

- **WHEN** agent issues a drain-state poll request
- **THEN** the HTTP request targets the v0.0.40 node endpoint and includes `X-SLURM-USER-NAME: <configured-or-default-user>` and `X-SLURM-USER-TOKEN: <workload-token>`

#### Scenario: Poll timeout exceeded

- **WHEN** node does not reach drained state within `POLL_TIMEOUT` (default 30m)
- **THEN** agent prints timeout error to stderr and exits with code 2

#### Scenario: slurmrestd unreachable during poll

- **WHEN** a single poll request fails (timeout or connection error)
- **THEN** agent logs warning and retries on next interval (does not exit immediately)