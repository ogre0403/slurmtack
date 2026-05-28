## Purpose

Define node-binding and placeholder-allocation requirements that keep switch executions unbound until MQ or Slurm supplies the concrete node and the workflow has exclusive control of it.

## Requirements

### Requirement: Slurm placeholder allocation binding

For `slurm` to `openstack` requests, the system SHALL submit a placeholder Slurm job that requests one exclusive GPU node before the execution becomes node-bound. When the execution includes a requested Slurm constraint or requested Slurm partition, the placeholder submission MUST include those selectors. The execution MUST remain in a pre-binding state until the placeholder job reveals the allocated node.

#### Scenario: Execution waits for allocated node using requested partition

- **WHEN** a `slurm_to_openstack` execution is created without a concrete node name and with `slurm_partition` set to `gpu-maint`
- **THEN** the system submits the placeholder job with that partition, transitions the execution to `awaiting_source_allocation`, and waits for the placeholder job allocation event

#### Scenario: Execution waits for allocated node without explicit partition

- **WHEN** a `slurm_to_openstack` execution is created without a concrete node name and without `slurm_partition`
- **THEN** the system submits the placeholder job without a partition selector, transitions the execution to `awaiting_source_allocation`, and waits for the placeholder job allocation event

### Requirement: Allocation event correlation

The placeholder job SHALL publish an allocation event containing `execution_id`, `job_id`, and `node_name`, and the daemon MUST ignore allocation events that do not match the active execution identity and state version.

#### Scenario: Matching allocation event binds execution

- **WHEN** the daemon receives an allocation event whose `execution_id` matches an execution in `awaiting_source_allocation`
- **THEN** it records the placeholder job ID, binds the execution to the allocated `node_name`, and transitions to `node_identified`

#### Scenario: Duplicate or stale allocation event is ignored

- **WHEN** the daemon receives an allocation event for an old execution or mismatched version
- **THEN** it discards the event without rebinding the active execution

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

### Requirement: Slurm reservation remains effective until detach handoff

After node identification, the system SHALL keep the allocated node isolated in Slurm until the daemon has drained the node, confirmed the placeholder job's terminal condition, and taken over the node for detach work. The switch MUST NOT proceed to node-bound mutation while other user jobs may still land on the node.

#### Scenario: Placeholder job guards the switch window

- **WHEN** the placeholder job has been allocated but the node is not yet drained
- **THEN** the system keeps the placeholder claim active and delays detach until drain conditions are satisfied

### Requirement: OpenStack-to-Slurm source clearance

For `openstack` to `slurm` requests, the system SHALL verify that no VM, migration, resize, or evacuation operation remains active on the node before source detachment begins.

#### Scenario: Compute workloads block switching

- **WHEN** the daemon finds resident or in-flight OpenStack workloads on the node
- **THEN** it fails the execution as `precheck_blocked` and does not mutate host ownership