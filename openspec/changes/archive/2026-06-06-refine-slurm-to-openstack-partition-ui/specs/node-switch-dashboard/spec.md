## MODIFIED Requirements

### Requirement: Trigger switch actions from the dashboard

The dashboard SHALL let an operator trigger `openstack_to_slurm` and `slurm_to_openstack` transitions by calling the existing switch creation API. For `openstack_to_slurm`, the dashboard MUST submit the selected `node_name` from the node card. For `slurm_to_openstack`, the dashboard MUST expose the action from partition-scoped controls rather than an individual node card because request-time node selection is unsupported. When the current partition selection is `All`, the dashboard MUST send `POST /v1/switches` with `direction=slurm_to_openstack` and no `node_name` or `slurm_partition`. When the current partition selection is a specific partition such as `gpu-maint`, the dashboard MUST send `POST /v1/switches` with `direction=slurm_to_openstack` and `slurm_partition=gpu-maint` without sending `node_name`. The dashboard MUST also expose cancellation for an active execution by calling the existing cancel endpoint.

#### Scenario: Launch openstack_to_slurm from a node card

- **WHEN** the operator starts an `openstack_to_slurm` action for node `gpu-01`
- **THEN** the dashboard sends `POST /v1/switches` with `direction=openstack_to_slurm` and `node_name=gpu-01`

#### Scenario: Launch slurm_to_openstack from All partition context

- **WHEN** the operator selects `All` in the partition panel and starts `slurm_to_openstack` from the partition-scoped action control
- **THEN** the dashboard sends `POST /v1/switches` with `direction=slurm_to_openstack` without sending `node_name` or `slurm_partition`

#### Scenario: Launch slurm_to_openstack from a specific partition context

- **WHEN** the operator selects partition `gpu-maint` and starts `slurm_to_openstack` from the partition-scoped action control
- **THEN** the dashboard sends `POST /v1/switches` with `direction=slurm_to_openstack` and `slurm_partition=gpu-maint` without sending `node_name`

#### Scenario: Slurm-owned node card does not imply request-time node targeting

- **WHEN** the dashboard renders a node card for a node whose available direction is `slurm_to_openstack`
- **THEN** the node card does not render a `slurm_to_openstack` action button
- **AND** the partition-scoped action control remains the place where the operator launches that workflow

#### Scenario: Cancel active execution from dashboard

- **WHEN** the operator clicks cancel for an execution currently shown as active on a node
- **THEN** the dashboard sends `POST /v1/switches/<id>/cancel` and reflects the accepted cancellation state
