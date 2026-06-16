## ADDED Requirements

### Requirement: Enforce optional cloud partition scope for dashboard inventory and switch creation

The system SHALL keep the current dashboard inventory and switch behavior when `SLURM_CLOUD_PARTITION` is unset. When `SLURM_CLOUD_PARTITION` is set, `GET /v1/dashboard/inventory` MUST limit the returned `partitions` and `nodes` to that configured partition only, and `POST /v1/switches` MUST enforce that same partition as the only valid cloud-switch scope. In scoped mode, `slurm_to_openstack` MUST treat the configured cloud partition as the effective partition when `slurm_partition` is omitted, MUST reject any different explicit `slurm_partition`, and `openstack_to_slurm` MUST reject any `node_name` that is not a member of the configured cloud partition.

#### Scenario: Query inventory remains unscoped when cloud partition is unset

- **WHEN** `SLURM_CLOUD_PARTITION` is unset and client sends GET `/v1/dashboard/inventory`
- **THEN** system returns HTTP 200 with discovered `partitions` and `nodes` derived from normal Slurm partition discovery

#### Scenario: Query inventory returns only the configured cloud partition

- **WHEN** `SLURM_CLOUD_PARTITION=gpu-cloud` is set and client sends GET `/v1/dashboard/inventory`
- **THEN** system returns HTTP 200 with only `gpu-cloud` in `partitions`
- **AND** every returned node belongs to `gpu-cloud`

#### Scenario: Query inventory rejects a conflicting partition filter in scoped mode

- **WHEN** `SLURM_CLOUD_PARTITION=gpu-cloud` is set and client sends GET `/v1/dashboard/inventory?partition=gpu-debug`
- **THEN** system returns HTTP 400
- **AND** the error indicates that the requested partition is outside the configured cloud partition scope

#### Scenario: Scoped slurm_to_openstack request omits explicit partition

- **WHEN** `SLURM_CLOUD_PARTITION=gpu-cloud` is set and client sends `POST /v1/switches` with `direction=slurm_to_openstack` and no `slurm_partition`
- **THEN** system accepts the request using `gpu-cloud` as the effective Slurm partition
- **AND** the created execution persists `requested_slurm_partition=gpu-cloud`

#### Scenario: Scoped slurm_to_openstack request rejects a different explicit partition

- **WHEN** `SLURM_CLOUD_PARTITION=gpu-cloud` is set and client sends `POST /v1/switches` with `direction=slurm_to_openstack` and `slurm_partition=gpu-debug`
- **THEN** system returns HTTP 400
- **AND** the error indicates that only `gpu-cloud` is allowed while scoped mode is enabled

#### Scenario: Scoped openstack_to_slurm request rejects a node outside the configured partition

- **WHEN** `SLURM_CLOUD_PARTITION=gpu-cloud` is set and client sends `POST /v1/switches` with `direction=openstack_to_slurm` for node `gpu-17`
- **AND** `gpu-17` is not a member of `gpu-cloud`
- **THEN** system returns HTTP 400
- **AND** no execution is created
