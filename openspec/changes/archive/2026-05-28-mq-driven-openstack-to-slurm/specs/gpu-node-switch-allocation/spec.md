## ADDED Requirements

### Requirement: OpenStack-to-Slurm MQ node binding

For `openstack_to_slurm` requests, the system SHALL accept the execution before a target node is bound and keep it in `awaiting_target_node` until a matching MQ node-selection event arrives. The node-selection event MUST contain `execution_id` and `node_name`. Once correlated, the system MUST record the node and transition the execution to `node_identified` before any lease acquisition or node-bound precheck begins.

#### Scenario: Matching node selection event binds execution
- **WHEN** an `openstack_to_slurm` execution is active in `awaiting_target_node` and the daemon receives a matching MQ node-selection event
- **THEN** it records the selected `node_name`, transitions the execution to `node_identified`, and only then allows lease acquisition and node-bound actions

#### Scenario: No node-bound work starts before MQ binding
- **WHEN** an `openstack_to_slurm` execution has been accepted but no matching MQ node-selection event has arrived
- **THEN** the system does not acquire a node lease, does not run node-bound prechecks, and does not mutate host ownership

#### Scenario: Stale node selection event is ignored
- **WHEN** the daemon receives a node-selection event for an execution that is already terminal or already past `awaiting_target_node`
- **THEN** it discards the event without rebinding the active execution