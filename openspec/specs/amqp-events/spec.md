## Purpose

Define the AMQP integration requirements for topology setup, event consumption, acknowledgement behavior, reconnection, message validation, and startup connection recovery.

## Requirements

### Requirement: Topology declaration on startup

The MQ module SHALL declare a durable topic exchange `gpu-switch.events` and four durable queues (`gpu-switch.requested`, `gpu-switch.node-selected`, `gpu-switch.allocation`, `gpu-switch.drained`) with appropriate bindings on daemon startup. Declaration MUST be idempotent.

#### Scenario: First startup (no existing topology)

- **WHEN** daemon starts and RabbitMQ has no existing exchange/queues
- **THEN** exchange and queues are created with correct bindings for `execution.requested`, `execution.node_selected`, `execution.allocation`, and `execution.drained`

#### Scenario: Restart with existing topology

- **WHEN** daemon restarts and topology already exists
- **THEN** declaration succeeds without error (idempotent)

### Requirement: Consume requested events

The MQ consumer SHALL subscribe to the `gpu-switch.requested` queue and process messages with schema `{"execution_id": string, "direction": string}`. On a valid message, it MUST admit the execution into the orchestrator control path without requiring a periodic store scan.

#### Scenario: Valid requested event for slurm_to_openstack

- **WHEN** a message arrives with `execution_id` matching a persisted `slurm_to_openstack` execution in `requested`
- **THEN** the daemon admits that execution into orchestration and begins the normal post-request workflow

#### Scenario: Valid requested event for openstack_to_slurm awaiting node binding

- **WHEN** a message arrives with `execution_id` matching a persisted `openstack_to_slurm` execution in `awaiting_target_node`
- **THEN** the daemon registers that execution for MQ-driven continuation and does not require a periodic store poll to discover it

#### Scenario: Duplicate or stale requested event

- **WHEN** a message arrives for an execution that is already terminal or already admitted past its request stage
- **THEN** the daemon acks and discards the message

### Requirement: Consume allocation events

The MQ consumer SHALL subscribe to the `gpu-switch.allocation` queue and process messages with schema `{"execution_id": string, "job_id": string, "node_name": string}`. On valid message, it MUST bind the execution to the node and transition from `awaiting_source_allocation` to `node_identified`.

#### Scenario: Valid allocation event

- **WHEN** a message arrives with execution_id matching an execution in `awaiting_source_allocation`
- **THEN** consumer binds node_name to the execution, transitions to `node_identified`, and acks the message

#### Scenario: Stale allocation event

- **WHEN** a message arrives with execution_id that is not in `awaiting_source_allocation` (already advanced or failed)
- **THEN** consumer acks and discards the message

#### Scenario: Unknown execution_id

- **WHEN** a message arrives with an execution_id that does not exist in the store
- **THEN** consumer acks and discards the message (no crash, no requeue)

### Requirement: Consume openstack_to_slurm node selection events

The MQ consumer SHALL subscribe to the `gpu-switch.node-selected` queue and process messages with schema `{"execution_id": string, "node_name": string}`. On a valid message, it MUST bind the execution to the selected node and transition from `awaiting_target_node` to `node_identified`.

#### Scenario: Valid node selection event

- **WHEN** a message arrives with `execution_id` matching an execution in `awaiting_target_node`
- **THEN** the consumer records `node_name`, transitions the execution to `node_identified`, and acks the message

#### Scenario: Duplicate node selection event

- **WHEN** a message arrives for an execution already past `awaiting_target_node`
- **THEN** the consumer acks and discards the message

#### Scenario: Unknown execution_id for node selection

- **WHEN** a message arrives with an `execution_id` that does not exist in the store
- **THEN** the consumer acks and discards the message

### Requirement: Consume node_drained events

The MQ consumer SHALL subscribe to the `gpu-switch.drained` queue and process messages with schema `{"execution_id": string, "node_name": string}`. On valid message, it MUST transition the execution from `source_quiescing` to `source_detached`.

#### Scenario: Valid drained event

- **WHEN** a message arrives with execution_id matching an execution in `source_quiescing`
- **THEN** consumer transitions to `source_detached` and acks the message

#### Scenario: Duplicate drained event

- **WHEN** a message arrives for an execution already past `source_quiescing`
- **THEN** consumer acks and discards the message

### Requirement: Manual acknowledgement

The MQ consumer SHALL use manual ack mode. Messages MUST be acked only after the state transition succeeds. On processing failure, messages MUST be nacked with requeue.

#### Scenario: Successful processing

- **WHEN** state transition succeeds
- **THEN** message is acked

#### Scenario: Processing failure (store error)

- **WHEN** state transition fails with a transient error (e.g., DB busy)
- **THEN** message is nacked with requeue=true

#### Scenario: Version conflict

- **WHEN** state transition fails with ErrVersionConflict
- **THEN** message is acked (another consumer/orchestrator already handled it)

### Requirement: Connection reconnect

The MQ consumer SHALL automatically reconnect to RabbitMQ if the connection is lost, using exponential backoff (starting at 1s, max 30s). Active executions in MQ-dependent states will resume when the consumer reconnects.

#### Scenario: RabbitMQ restart

- **WHEN** RabbitMQ connection drops
- **THEN** consumer retries connection with backoff and resumes consuming once reconnected

#### Scenario: Shutdown during reconnect

- **WHEN** context is cancelled while consumer is in reconnect backoff
- **THEN** consumer exits without further retry attempts

### Requirement: Message validation

The MQ consumer SHALL validate incoming message JSON against the expected schema. Malformed messages MUST be acked and discarded (not requeued indefinitely).

#### Scenario: Malformed JSON

- **WHEN** a message with invalid JSON arrives
- **THEN** consumer logs a warning, acks the message, and continues

#### Scenario: Missing required field

- **WHEN** a message is valid JSON but missing `execution_id`
- **THEN** consumer logs a warning, acks the message, and continues

### Requirement: Startup connection recovery

When `AMQP_URL` is configured and RabbitMQ is not yet ready during daemon startup, the system SHALL keep retrying MQ activation with bounded exponential backoff until MQ becomes available or the daemon shuts down. MQ activation MUST include connection establishment, topology declaration, and consumer startup.

#### Scenario: RabbitMQ becomes ready after the daemon starts

- **WHEN** the daemon starts with `AMQP_URL` configured and the first MQ activation attempt fails because RabbitMQ is not yet ready
- **THEN** the daemon keeps retrying MQ activation, starts consuming from `gpu-switch.requested`, `gpu-switch.node-selected`, `gpu-switch.allocation`, and `gpu-switch.drained` once RabbitMQ becomes available, and does not require a manual process restart

#### Scenario: Shutdown interrupts startup retry

- **WHEN** the daemon is retrying MQ activation during startup and receives shutdown
- **THEN** the daemon stops further retry attempts and exits cleanly without leaving a partial MQ startup loop running

#### Scenario: MQ remains optional when not configured

- **WHEN** the daemon starts without `AMQP_URL`
- **THEN** the daemon does not enter the MQ startup retry loop and continues running without MQ integration
