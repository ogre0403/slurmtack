## MODIFIED Requirements

### Requirement: Slurm placeholder allocation binding

For `slurm` to `openstack` requests, the system SHALL submit a placeholder Slurm job that requests one exclusive GPU node before the execution becomes node-bound. When the execution includes a requested Slurm constraint or requested Slurm partition, the placeholder submission MUST include those selectors. The execution MUST remain in a pre-binding state until the placeholder job reveals the allocated node.

#### Scenario: Execution waits for allocated node using requested partition

- **WHEN** a `slurm_to_openstack` execution is created without a concrete node name and with `slurm_partition` set to `gpu-maint`
- **THEN** the system submits the placeholder job with that partition, transitions the execution to `awaiting_source_allocation`, and waits for the placeholder job allocation event

#### Scenario: Execution waits for allocated node without explicit partition

- **WHEN** a `slurm_to_openstack` execution is created without a concrete node name and without `slurm_partition`
- **THEN** the system submits the placeholder job without a partition selector, transitions the execution to `awaiting_source_allocation`, and waits for the placeholder job allocation event