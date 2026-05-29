## MODIFIED Requirements

### Requirement: OpenStack-to-Slurm MQ node binding

For `openstack_to_slurm` requests, the system SHALL require the API request to include the target `node_name`, persist the execution in `awaiting_target_node`, and publish a matching MQ node-selection event containing `execution_id` and `node_name`. Once that event is correlated, the system MUST record the node and transition the execution to `node_identified` before any lease acquisition or node-bound precheck begins.

#### Scenario: API-submitted node name is published and then binds execution

- **WHEN** an operator submits `POST /v1/switches` for `openstack_to_slurm` with `node_name` `gpu-01`
- **THEN** the system persists the execution, publishes the matching node-selection event, and later records `gpu-01` on the execution when that event is consumed

#### Scenario: No node-bound work starts before MQ binding

- **WHEN** an `openstack_to_slurm` execution has been accepted and persisted in `awaiting_target_node` but the published node-selection event has not yet been consumed
- **THEN** the system does not acquire a node lease, does not run node-bound prechecks, and does not mutate host ownership

#### Scenario: Stale node selection event is ignored

- **WHEN** the daemon receives a node-selection event for an execution that is already terminal or already past `awaiting_target_node`
- **THEN** it discards the event without rebinding the active execution
