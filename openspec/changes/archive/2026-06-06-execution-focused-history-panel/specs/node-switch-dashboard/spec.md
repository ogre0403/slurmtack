## MODIFIED Requirements

### Requirement: Trigger switch actions from the dashboard

The dashboard SHALL let an operator trigger `openstack_to_slurm` and `slurm_to_openstack` transitions by calling the existing switch creation API. For `openstack_to_slurm`, the dashboard MUST submit the selected `node_name` from the node card. For `slurm_to_openstack`, the dashboard MUST expose the action from partition-scoped controls rather than an individual node card because request-time node selection is unsupported. When the current partition selection is `All`, the dashboard MUST send `POST /v1/switches` with `direction=slurm_to_openstack` and no `node_name` or `slurm_partition`. When the current partition selection is a specific partition such as `gpu-maint`, the dashboard MUST send `POST /v1/switches` with `direction=slurm_to_openstack` and `slurm_partition=gpu-maint` without sending `node_name`. The dashboard MUST also expose cancellation for an active execution by calling the existing cancel endpoint from both the node summary area and the execution-focused right-side panel.

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

#### Scenario: Cancel active execution from a node summary

- **WHEN** the operator clicks cancel for an execution currently shown as active on a node
- **THEN** the dashboard sends `POST /v1/switches/<id>/cancel`
- **AND** it reflects the accepted cancellation state

#### Scenario: Cancel active execution from the execution panel

- **WHEN** the execution-focused right-side panel renders an execution whose `overall_status` is `active`
- **AND** the operator clicks cancel for that execution
- **THEN** the dashboard sends `POST /v1/switches/<id>/cancel`
- **AND** it refreshes the execution row and selected detail view to reflect the accepted cancellation state

### Requirement: Browse execution history and inspect step timelines

The dashboard SHALL provide an execution-focused right-side panel with a paginated execution list and a drilldown panel for an individual execution. The execution list MUST include recent active executions and prior executions returned by the API, show 10 executions per page by default, and display each execution's direction, node binding when known, `current_state`, `overall_status`, and request time. Selecting an execution MUST show the execution summary and current state in the detail panel, and the drilldown panel MUST render the ordered step timeline returned for the selected execution. The dashboard MAY continue to offer node and outcome filters, but changing filters MUST refresh the execution list and reset pagination to the first page.

#### Scenario: Inspect execution details from the execution list

- **WHEN** the operator selects an execution from the right-side execution list
- **THEN** the dashboard shows the execution summary, its current state, and its ordered step records in the detail panel

#### Scenario: Browse the next page of executions

- **WHEN** the execution list contains more than 10 matching executions
- **AND** the operator navigates to the next page
- **THEN** the dashboard shows the next 10 executions from the list query
- **AND** it keeps the execution rows ordered consistently with the API response

#### Scenario: Browse the previous page of executions

- **WHEN** the operator has navigated beyond the first execution page
- **AND** the operator navigates back to the previous page
- **THEN** the dashboard restores the immediately preceding set of 10 execution rows

#### Scenario: Active execution remains actionable in the list

- **WHEN** the execution list includes an execution whose `overall_status` is `active`
- **THEN** the row shows the execution's current state
- **AND** the row includes a cancel control without requiring the operator to open the detail panel first

#### Scenario: Filter execution list by node or outcome

- **WHEN** the operator applies a filter such as `node=gpu-01` or `status=failed`
- **THEN** the dashboard refreshes the execution list using those filters
- **AND** it resets the paginated view to the first page of matching executions
