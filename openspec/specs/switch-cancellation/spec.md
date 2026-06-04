## Purpose

Define requirements for cancelling an in-progress switch execution that is waiting in an approved wait state.

## Requirements

### Requirement: Cancellation is only claimed from approved wait states

The system SHALL accept operator cancellation only when an execution is currently in `awaiting_target_node`, `awaiting_source_allocation`, or `source_quiescing`. When cancellation is accepted, the system MUST persist the original wait state as cancellation context and transition the execution to `cancelling` before any cleanup action begins.

#### Scenario: Openstack_to_slurm cancellation is accepted while waiting for target-node admission

- **WHEN** an `openstack_to_slurm` execution is in `awaiting_target_node`
- **AND** an operator submits `POST /v1/switches/:id/cancel`
- **THEN** the system records `awaiting_target_node` as the cancellation source state
- **AND** the execution transitions to `cancelling`

#### Scenario: Slurm_to_openstack cancellation is accepted while waiting for placeholder allocation

- **WHEN** a `slurm_to_openstack` execution is in `awaiting_source_allocation`
- **AND** an operator submits `POST /v1/switches/:id/cancel`
- **THEN** the system records `awaiting_source_allocation` as the cancellation source state
- **AND** the execution transitions to `cancelling`

#### Scenario: Cancellation is rejected outside approved wait states

- **WHEN** an execution is in a non-approved active state such as `rebooting`
- **AND** an operator submits `POST /v1/switches/:id/cancel`
- **THEN** the system rejects the cancellation request
- **AND** the execution remains in its original state

### Requirement: Cancellation cleanup depends on direction and claimed wait state

After an execution enters `cancelling`, the system SHALL run cleanup actions based on the persisted cancellation source state and direction before finalizing the cancellation.

#### Scenario: Awaiting_target_node cancellation needs no external cleanup

- **WHEN** an `openstack_to_slurm` execution is in `cancelling`
- **AND** its recorded cancellation source state is `awaiting_target_node`
- **THEN** the system performs no Slurm, OpenStack, or lease cleanup
- **AND** it proceeds directly toward terminal cancellation

#### Scenario: Awaiting_source_allocation cancellation cancels the placeholder job

- **WHEN** a `slurm_to_openstack` execution is in `cancelling`
- **AND** its recorded cancellation source state is `awaiting_source_allocation`
- **AND** a `placeholder_job_id` is present on the execution
- **THEN** the system calls the Slurm job-cancellation API for that placeholder job before finalizing cancellation

#### Scenario: Slurm_to_openstack source_quiescing cancellation restores Slurm ownership

- **WHEN** a `slurm_to_openstack` execution is in `cancelling`
- **AND** its recorded cancellation source state is `source_quiescing`
- **THEN** the system resumes the Slurm node, cancels the placeholder job when present, and releases the node lease before finalizing cancellation

#### Scenario: Openstack_to_slurm source_quiescing cancellation restores OpenStack ownership

- **WHEN** an `openstack_to_slurm` execution is in `cancelling`
- **AND** its recorded cancellation source state is `source_quiescing`
- **THEN** the system re-enables the OpenStack compute service and releases the node lease before finalizing cancellation

### Requirement: Cancellation ends in a distinct terminal outcome

The system SHALL transition a successfully cleaned-up execution from `cancelling` to terminal `cancelled`. A cancelled execution MUST report `overall_status` as failed, MUST record a final error code identifying user cancellation, and MUST NOT resume normal switch progression. If cancellation cleanup fails, the system MUST stop the normal workflow and terminalize the execution through the non-destructive failure path with a cancellation-specific error.

#### Scenario: Successful cleanup reaches cancelled

- **WHEN** a `cancelling` execution completes its required cleanup actions
- **THEN** the system transitions the execution to `cancelled`
- **AND** it records `final_error_code` as `cancelled_by_user`
- **AND** `overall_status` becomes `failed`

#### Scenario: Cleanup failure becomes terminal failure

- **WHEN** a `cancelling` execution cannot complete one of its required cleanup actions
- **THEN** the system transitions the execution to `failed_non_destructive`
- **AND** it records a cancellation-specific terminal error code such as `cancel_cleanup_failed`
- **AND** the normal switch workflow does not resume afterward
