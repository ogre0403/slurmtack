## ADDED Requirements

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

The agent SHALL poll `GET /slurm/v0.0.40/node/{hostname}` at the configured interval (default 5s) to check if the node has entered the Slurm drain state required for the switch handoff. Each poll request MUST include `X-SLURM-USER-NAME` and `X-SLURM-USER-TOKEN` headers derived from the configured workload Slurm identity. Drain completion MUST be determined from Slurm state tokens rather than exact full-string matches. The agent MUST treat `drain`, `drained`, `drained*`, `down`, and `down*` tokens as satisfying the drain wait even when they appear inside a composite state such as `MIXED+DRAIN`. The agent MUST NOT enforce an internal overall poll timeout; the placeholder job runtime MUST instead be governed by Slurm job control such as partition time limits, cancellation, or successful drain completion.

#### Scenario: Node drains within polling window

- **WHEN** slurmrestd reports node state as `drained` during polling
- **THEN** agent proceeds to publish node_drained event

#### Scenario: Composite drain state completes polling

- **WHEN** slurmrestd reports node state as `MIXED+DRAIN`
- **THEN** agent considers the drain wait satisfied and proceeds to publish node_drained event

#### Scenario: Poll request uses Slurm identity headers

- **WHEN** agent issues a drain-state poll request
- **THEN** the HTTP request targets the v0.0.40 node endpoint and includes `X-SLURM-USER-NAME: <configured-or-default-user>` and `X-SLURM-USER-TOKEN: <workload-token>`

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

The agent SHALL use specific exit codes to communicate outcome to Slurm and the daemon: 0 (success), 1 (startup failure or local shutdown), and 3 (MQ publish failure). The agent MUST NOT emit a dedicated poll-timeout exit code because drain waiting is bounded by Slurm job policy rather than an internal deadline.

#### Scenario: Successful completion

- **WHEN** both events are published and drain is confirmed
- **THEN** agent exits with code 0

#### Scenario: Startup failure

- **WHEN** required env vars are missing, initial connections fail, or the process is cancelled before completion
- **THEN** agent exits with code 1

#### Scenario: MQ publish failure

- **WHEN** allocation or drained event publishing cannot be confirmed after the documented retry path
- **THEN** agent exits with code 3

### Requirement: Structured stdout logging

The agent SHALL write structured log lines (JSON) to stdout for observability. Each log line MUST include timestamp, level, execution_id, and message. Slurm captures stdout in the job output file.

#### Scenario: Normal operation

- **WHEN** agent runs through its lifecycle
- **THEN** stdout contains JSON log lines for: startup, allocation_event published, each poll result, node_drained published, exit

#### Scenario: Error condition

- **WHEN** agent encounters an error
- **THEN** the error is logged as a JSON line with level "error" before exiting
