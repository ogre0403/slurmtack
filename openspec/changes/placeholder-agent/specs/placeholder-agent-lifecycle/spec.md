## ADDED Requirements

### Requirement: Environment variable configuration

The agent SHALL read all configuration from environment variables. It MUST exit with code 1 if any required variable (`EXECUTION_ID`, `AMQP_URL`, `SLURM_API_URL`, `SLURM_JWT_TOKEN`) is missing or empty.

#### Scenario: All required vars present

- **WHEN** agent starts with all required environment variables set
- **THEN** agent proceeds with startup

#### Scenario: Missing EXECUTION_ID

- **WHEN** agent starts without EXECUTION_ID set
- **THEN** agent prints error to stderr and exits with code 1

#### Scenario: Missing AMQP_URL

- **WHEN** agent starts without AMQP_URL set
- **THEN** agent prints error to stderr and exits with code 1

### Requirement: Publish allocation event

The agent SHALL discover its hostname via `os.Hostname()`, connect to RabbitMQ, and publish an allocation_event message with routing key `execution.allocation` to exchange `gpu-switch.events`. The message MUST contain `execution_id`, `job_id` (from SLURM_JOB_ID env), and `node_name`.

#### Scenario: Successful publish

- **WHEN** agent starts on allocated node with valid AMQP_URL
- **THEN** agent publishes allocation_event with correct execution_id, job_id, and discovered hostname

#### Scenario: MQ connection failure

- **WHEN** agent cannot connect to RabbitMQ
- **THEN** agent prints error to stderr and exits with code 3

#### Scenario: Publish confirmation failure

- **WHEN** RabbitMQ rejects the publish (e.g., exchange does not exist)
- **THEN** agent prints error to stderr and exits with code 3

### Requirement: Poll slurmrestd for drain state

The agent SHALL poll `GET /slurm/v0.0.38/node/{hostname}` at the configured interval (default 5s) to check if the node has reached a drained state. Drained states are: `drained`, `drained*`, `down`, `down*`.

#### Scenario: Node drains within timeout

- **WHEN** slurmrestd reports node state as "drained" during polling
- **THEN** agent proceeds to publish node_drained event

#### Scenario: Node state is "drained*"

- **WHEN** slurmrestd reports node state as "drained*" (drain pending jobs complete)
- **THEN** agent considers this as drained and proceeds

#### Scenario: Poll timeout exceeded

- **WHEN** node does not reach drained state within POLL_TIMEOUT (default 30m)
- **THEN** agent prints timeout error to stderr and exits with code 2

#### Scenario: slurmrestd unreachable during poll

- **WHEN** a single poll request fails (timeout or connection error)
- **THEN** agent logs warning and retries on next interval (does not exit immediately)

### Requirement: Publish node_drained event

After confirming drain state, the agent SHALL publish a node_drained message with routing key `execution.drained` to exchange `gpu-switch.events`. The message MUST contain `execution_id` and `node_name`.

#### Scenario: Successful drained publish

- **WHEN** drain is confirmed and MQ connection is alive
- **THEN** agent publishes node_drained event and exits with code 0

#### Scenario: MQ connection lost before drained publish

- **WHEN** drain is confirmed but MQ connection was lost
- **THEN** agent attempts one reconnect; if failed, exits with code 3

### Requirement: Exit codes

The agent SHALL use specific exit codes to communicate outcome to Slurm and the daemon: 0 (success), 1 (startup failure), 2 (poll timeout), 3 (MQ publish failure).

#### Scenario: Successful completion

- **WHEN** both events are published and drain is confirmed
- **THEN** agent exits with code 0

#### Scenario: Startup failure

- **WHEN** required env vars are missing or initial connections fail
- **THEN** agent exits with code 1

#### Scenario: Drain timeout

- **WHEN** poll loop exceeds POLL_TIMEOUT
- **THEN** agent exits with code 2

### Requirement: Structured stdout logging

The agent SHALL write structured log lines (JSON) to stdout for observability. Each log line MUST include timestamp, level, execution_id, and message. Slurm captures stdout in the job output file.

#### Scenario: Normal operation

- **WHEN** agent runs through its lifecycle
- **THEN** stdout contains JSON log lines for: startup, allocation_event published, each poll result, node_drained published, exit

#### Scenario: Error condition

- **WHEN** agent encounters an error
- **THEN** the error is logged as a JSON line with level "error" before exiting
