## MODIFIED Requirements

### Requirement: Slurm placeholder allocation binding

For `slurm` to `openstack` requests, the system SHALL resolve one effective workload Slurm identity before placeholder submission and submit a placeholder Slurm job that requests one exclusive GPU node before the execution becomes node-bound. When the execution includes a requested Slurm constraint, requested Slurm partition, or requested Slurm account, the placeholder submission MUST include those selectors. The execution MUST remain in a pre-binding state until the placeholder job reveals the allocated node. If the placeholder job reaches any terminal state before an allocation event is received, the system MUST stop waiting, persist an operator-visible failure reason, and fail the execution as a non-destructive pre-binding failure.

#### Scenario: Execution waits for allocated node using requested partition and account

- **WHEN** a `slurm_to_openstack` execution is created without a concrete node name and with `slurm_partition` set to `gpu-maint` and `slurm_account` set to `proj-123`
- **THEN** the system submits the placeholder job with that partition and account, transitions the execution to `awaiting_source_allocation`, and waits for the placeholder job allocation event

#### Scenario: Execution waits for allocated node using request-scoped workload credentials

- **WHEN** a `slurm_to_openstack` execution is created without a concrete node name and with request-scoped `slurm_user` and `slurm_user_token`
- **THEN** the system submits the placeholder job using that workload identity, transitions the execution to `awaiting_source_allocation`, and waits for the placeholder job allocation event

#### Scenario: Execution waits for allocated node without explicit partition or account

- **WHEN** a `slurm_to_openstack` execution is created without a concrete node name and without `slurm_partition` or `slurm_account`
- **THEN** the system submits the placeholder job without those selectors, transitions the execution to `awaiting_source_allocation`, and waits for the placeholder job allocation event

#### Scenario: Placeholder job fails before allocation is published

- **WHEN** a `slurm_to_openstack` execution is already in `awaiting_source_allocation`
- **AND** its recorded placeholder job reaches terminal Slurm state `FAILED` before any allocation event is received
- **THEN** the system closes the allocation wait as failed
- **AND** it transitions the execution to `failed_non_destructive`
- **AND** it records a readable failure reason that mentions the placeholder job and observed state

#### Scenario: Placeholder job ends without ever binding a node

- **WHEN** a `slurm_to_openstack` execution is already in `awaiting_source_allocation`
- **AND** its recorded placeholder job reaches terminal Slurm state `COMPLETED` before any allocation event is received
- **THEN** the system treats the wait as failed rather than successful
- **AND** it transitions the execution to `failed_non_destructive`
- **AND** it records that the placeholder job ended before node allocation was bound to the execution
