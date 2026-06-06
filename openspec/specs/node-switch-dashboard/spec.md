## Requirements

### Requirement: Display partition-scoped node inventory

The system SHALL provide an operator dashboard at `/` that loads a same-origin inventory read model and presents nodes grouped by Slurm partition. The primary inventory view MUST only include nodes discovered from Slurm partition membership, and the dashboard MUST allow the operator to scope the visible grid to a selected partition or to all discovered partitions.

#### Scenario: Load dashboard inventory for all partitions

- **WHEN** an operator opens the dashboard and the same-origin inventory request succeeds
- **THEN** the page shows the discovered Slurm partitions and a node inventory derived from those partitions

#### Scenario: Filter dashboard by selected partition

- **WHEN** the operator selects a specific partition such as `gpu-maint`
- **THEN** the dashboard shows only nodes that belong to that partition while preserving each node's canonical status fields

### Requirement: Show ownership and readiness summary per node

The dashboard SHALL display a status card or row for each discovered node that includes `node_name`, `partitions`, a normalized owner classification, a Slurm summary, an OpenStack summary, any active execution summary, and the most recent completed or failed execution summary. The owner classification MUST support at least `slurm`, `openstack`, `switching`, `conflict`, and `unknown`.

#### Scenario: Node is actively switching

- **WHEN** the inventory response marks a node with an active execution
- **THEN** the dashboard shows that node as `switching` and includes the active execution identifier and current state

#### Scenario: Node ownership is ambiguous

- **WHEN** the inventory response classifies a node as `conflict` or `unknown`
- **THEN** the dashboard renders that classification distinctly instead of implying ownership by either Slurm or OpenStack

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

### Requirement: Browse execution history and inspect step timelines

The dashboard SHALL provide an execution history view with filters and a drilldown panel for an individual execution. The history view MUST show recent executions returned by the API, and the drilldown panel MUST render the ordered step timeline returned for the selected execution.

#### Scenario: Inspect execution details from history

- **WHEN** the operator selects an execution from the history list
- **THEN** the dashboard shows the execution summary and its ordered step records in the detail panel

#### Scenario: Filter history by node or outcome

- **WHEN** the operator applies a history filter such as `node=gpu-01` or `status=failed`
- **THEN** the dashboard refreshes the history list using those filters and shows only matching executions
