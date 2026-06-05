## MODIFIED Requirements

### Requirement: Query execution status

The system SHALL return execution details via `GET /v1/switches/:id` including current state, direction, node name when bound, overall status, timestamps, and error information if failed. The response MUST also include the execution metadata already persisted by the daemon that is required for operator drilldown, including `state_version`, `desired_owner`, `previous_owner`, lock timing, requested Slurm constraint or partition, placeholder job identifier when present, allocation event timestamp when present, and `cancellation_source_state` when cancellation was claimed.

#### Scenario: Query existing execution

- **WHEN** client sends GET `/v1/switches/:id` for an existing execution
- **THEN** system returns HTTP 200 with execution details including `id`, `node_name`, `direction`, `current_state`, `overall_status`, `requested_at`, `requested_by`, and the persisted operator drilldown metadata

#### Scenario: Query openstack_to_slurm execution before node binding consumption

- **WHEN** client sends GET `/v1/switches/:id` for an `openstack_to_slurm` execution that has been accepted but not yet advanced by the MQ consumer
- **THEN** system returns HTTP 200 with `current_state` set to `awaiting_target_node`
- **AND** `node_name` contains the API-supplied target node

#### Scenario: Query non-existent execution

- **WHEN** client sends GET `/v1/switches/:id` for a non-existent ID
- **THEN** system returns HTTP 404 with error message

### Requirement: List executions

The system SHALL return a list of executions via `GET /v1/switches` with optional query filters for `node`, `status`, and `direction`. The endpoint MUST also support history pagination with `limit` and `before` query parameters, where `before` is an RFC3339 timestamp used to request older executions. Results MUST be ordered by `requested_at` descending and each execution summary MUST include the fields needed by the dashboard history view, including `id`, `node_name`, `direction`, `current_state`, `overall_status`, `requested_at`, `requested_by`, and terminal error summary fields when present.

#### Scenario: List all executions

- **WHEN** client sends GET `/v1/switches`
- **THEN** system returns HTTP 200 with an array of execution summaries ordered from newest to oldest

#### Scenario: Filter by node name

- **WHEN** client sends GET `/v1/switches?node=gpu-01`
- **THEN** system returns HTTP 200 with only executions matching `node_name` `gpu-01`

#### Scenario: Filter by overall status

- **WHEN** client sends GET `/v1/switches?status=active`
- **THEN** system returns HTTP 200 with only executions with `overall_status` `active`

#### Scenario: Filter by direction

- **WHEN** client sends GET `/v1/switches?direction=openstack_to_slurm`
- **THEN** system returns HTTP 200 with only executions whose `direction` is `openstack_to_slurm`

#### Scenario: Page through older history

- **WHEN** client sends GET `/v1/switches?limit=20&before=2026-06-05T09:00:00Z`
- **THEN** system returns at most 20 execution summaries requested before that timestamp

## ADDED Requirements

### Requirement: Query execution step timeline

The system SHALL return an ordered step timeline via `GET /v1/switches/:id/steps` for an existing execution. Each step item MUST include `sequence`, `step_name`, `host`, `started_at`, `ended_at`, `status`, `retry_count`, `exit_code`, `error_class`, `command_id`, and any persisted stdout, stderr, or snapshot paths.

#### Scenario: Query step timeline for existing execution

- **WHEN** client sends GET `/v1/switches/:id/steps` for an existing execution
- **THEN** system returns HTTP 200 with that execution's step records ordered by ascending `sequence`

#### Scenario: Query step timeline for unknown execution

- **WHEN** client sends GET `/v1/switches/:id/steps` for a non-existent execution ID
- **THEN** system returns HTTP 404 with an error indicating that the execution was not found

### Requirement: Query partition-scoped node inventory

The system SHALL expose `GET /v1/dashboard/inventory` as an authenticated read-model endpoint for the operator dashboard. The endpoint MUST derive the displayed node set from Slurm partition membership, return the discovered partitions, and return one normalized node summary per discovered node. Each node summary MUST include `node_name`, `partitions`, a normalized owner classification, a Slurm status summary, an OpenStack status summary, any active execution summary, and the most recent execution summary. The endpoint MUST support an optional `partition` query parameter that limits the response to a selected Slurm partition.

#### Scenario: Query inventory for all partitions

- **WHEN** client sends GET `/v1/dashboard/inventory`
- **THEN** system returns HTTP 200 with discovered `partitions` and `nodes` derived from Slurm partition membership

#### Scenario: Query inventory for one partition

- **WHEN** client sends GET `/v1/dashboard/inventory?partition=gpu-maint`
- **THEN** system returns HTTP 200 with only the selected partition in scope and only nodes that belong to `gpu-maint`

#### Scenario: Inventory shows active execution and last execution context

- **WHEN** a discovered node has an active execution and a previous completed execution
- **THEN** the node summary includes the active execution identifier and state plus the most recent historical execution summary
