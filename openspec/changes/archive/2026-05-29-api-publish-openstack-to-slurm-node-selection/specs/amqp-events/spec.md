## MODIFIED Requirements

### Requirement: Consume openstack_to_slurm node selection events

The MQ consumer SHALL subscribe to the `gpu-switch.node-selected` queue and process messages with schema `{"execution_id": string, "node_name": string}`. For API-created `openstack_to_slurm` requests, the API/service path MUST publish this event immediately after execution persistence using the `node_name` supplied in `POST /v1/switches`. On a valid message, the consumer MUST bind the execution to the selected node and transition from `awaiting_target_node` to `node_identified`.

#### Scenario: API publishes node selection after openstack_to_slurm persistence

- **WHEN** the API accepts `POST /v1/switches` for `openstack_to_slurm` with `node_name` `gpu-01`
- **THEN** the system persists the execution first and then publishes `execution.node_selected` with that execution ID and `node_name` `gpu-01`

#### Scenario: Valid node selection event

- **WHEN** a message arrives with `execution_id` matching an execution in `awaiting_target_node`
- **THEN** the consumer records `node_name`, transitions the execution to `node_identified`, and acks the message

#### Scenario: Duplicate node selection event

- **WHEN** a message arrives for an execution already past `awaiting_target_node`
- **THEN** the consumer acks and discards the message

#### Scenario: Unknown execution_id for node selection

- **WHEN** a message arrives with an `execution_id` that does not exist in the store
- **THEN** the consumer acks and discards the message
