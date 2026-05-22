## ADDED Requirements

### Requirement: Slurm placeholder allocation binding
For `slurm` to `openstack` requests, the system SHALL submit a placeholder Slurm job that requests one exclusive GPU node before the execution becomes node-bound. The execution MUST remain in a pre-binding state until the placeholder job reveals the allocated node.

#### Scenario: Execution waits for allocated node
- **WHEN** a Slurm-to-OpenStack execution is created without a concrete node name
- **THEN** the system transitions the execution to `awaiting_source_allocation` and waits for the placeholder job allocation event

### Requirement: Allocation event correlation
The placeholder job SHALL publish an allocation event containing `execution_id`, `job_id`, and `node_name`, and the daemon MUST ignore allocation events that do not match the active execution identity and state version.

#### Scenario: Matching allocation event binds execution
- **WHEN** the daemon receives an allocation event whose `execution_id` matches an execution in `awaiting_source_allocation`
- **THEN** it records the placeholder job ID, binds the execution to the allocated `node_name`, and transitions to `node_identified`

#### Scenario: Duplicate or stale allocation event is ignored
- **WHEN** the daemon receives an allocation event for an old execution or mismatched version
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