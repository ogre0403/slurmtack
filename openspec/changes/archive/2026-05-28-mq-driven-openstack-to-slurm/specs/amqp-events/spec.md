## MODIFIED Requirements

### Requirement: Topology declaration on startup

The MQ module SHALL declare a durable topic exchange `gpu-switch.events` and four durable queues (`gpu-switch.requested`, `gpu-switch.node-selected`, `gpu-switch.allocation`, `gpu-switch.drained`) with appropriate bindings on daemon startup. Declaration MUST be idempotent.

#### Scenario: First startup (no existing topology)
- **WHEN** daemon starts and RabbitMQ has no existing exchange/queues
- **THEN** exchange and queues are created with correct bindings for `execution.requested`, `execution.node_selected`, `execution.allocation`, and `execution.drained`

#### Scenario: Restart with existing topology
- **WHEN** daemon restarts and topology already exists
- **THEN** declaration succeeds without error (idempotent)

## ADDED Requirements

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