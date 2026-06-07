## MODIFIED Requirements

### Requirement: Slurm placeholder allocation binding

For `slurm` to `openstack` requests, the system SHALL resolve one effective workload Slurm identity before placeholder submission and submit a placeholder Slurm job that requests one exclusive GPU node before the execution becomes node-bound. When the execution includes a requested Slurm constraint, requested Slurm partition, or requested Slurm account, the placeholder submission MUST include those selectors. The execution MUST remain in a pre-binding state until the placeholder job reveals the allocated node.

#### Scenario: Execution waits for allocated node using requested partition and account

- **WHEN** a `slurm_to_openstack` execution is created without a concrete node name and with `slurm_partition` set to `gpu-maint` and `slurm_account` set to `proj-123`
- **THEN** the system submits the placeholder job with that partition and account, transitions the execution to `awaiting_source_allocation`, and waits for the placeholder job allocation event

#### Scenario: Execution waits for allocated node using request-scoped workload credentials

- **WHEN** a `slurm_to_openstack` execution is created without a concrete node name and with request-scoped `slurm_user` and `slurm_user_token`
- **THEN** the system submits the placeholder job using that workload identity, transitions the execution to `awaiting_source_allocation`, and waits for the placeholder job allocation event

#### Scenario: Execution waits for allocated node without explicit partition or account

- **WHEN** a `slurm_to_openstack` execution is created without a concrete node name and without `slurm_partition` or `slurm_account`
- **THEN** the system submits the placeholder job without those selectors, transitions the execution to `awaiting_source_allocation`, and waits for the placeholder job allocation event
