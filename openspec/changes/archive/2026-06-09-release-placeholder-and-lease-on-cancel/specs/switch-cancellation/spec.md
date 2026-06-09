## MODIFIED Requirements

### Requirement: Cancellation cleanup depends on direction and claimed wait state

After an execution enters `cancelling`, the system SHALL run cleanup actions based on the persisted cancellation source state and direction before finalizing the cancellation. The system MUST also inspect the resources currently associated with the execution during cleanup: for `slurm_to_openstack`, any non-empty `placeholder_job_id` MUST be cancelled before finalizing cancellation, and any lease record currently held by the execution for its `node_name` MUST be released before finalizing cancellation. Source-state-specific rollback actions still apply only when the recorded cancellation source state requires them.

#### Scenario: Awaiting_target_node cancellation needs no external cleanup when no resources were claimed

- **WHEN** an `openstack_to_slurm` execution is in `cancelling`
- **AND** its recorded cancellation source state is `awaiting_target_node`
- **AND** it has no placeholder job and no execution-owned lease
- **THEN** the system performs no Slurm, OpenStack, or lease cleanup
- **AND** it proceeds directly toward terminal cancellation

#### Scenario: Awaiting_source_allocation cancellation removes any placeholder job and lease already attached to the execution

- **WHEN** a `slurm_to_openstack` execution is in `cancelling`
- **AND** its recorded cancellation source state is `awaiting_source_allocation`
- **AND** a `placeholder_job_id` is present on the execution
- **AND** the execution currently owns a lease for its allocated `node_name`
- **THEN** the system calls the Slurm job-cancellation API for that placeholder job before finalizing cancellation
- **AND** the system releases the execution-owned lease before finalizing cancellation

#### Scenario: Slurm_to_openstack source_quiescing cancellation restores Slurm ownership and clears execution-owned resources

- **WHEN** a `slurm_to_openstack` execution is in `cancelling`
- **AND** its recorded cancellation source state is `source_quiescing`
- **THEN** the system resumes the Slurm node
- **AND** the system cancels the placeholder job when present
- **AND** the system releases the node lease when held by the execution before finalizing cancellation

#### Scenario: Openstack_to_slurm source_quiescing cancellation restores OpenStack ownership

- **WHEN** an `openstack_to_slurm` execution is in `cancelling`
- **AND** its recorded cancellation source state is `source_quiescing`
- **THEN** the system re-enables the OpenStack compute service
- **AND** the system releases the node lease when held by the execution before finalizing cancellation

#### Scenario: Cancellation tolerates resources that were already removed

- **WHEN** a `cancelling` execution reaches cleanup
- **AND** its placeholder job or lease record was already removed by a prior retry or manual action
- **THEN** the system treats those resources as already cleaned
- **AND** it continues toward terminal `cancelled` instead of failing cleanup only because the resource was absent
