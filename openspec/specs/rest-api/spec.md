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

## MODIFIED Requirements

### Requirement: Query execution step timeline

The system SHALL return a durable execution step timeline via `GET /v1/switches/:id/steps` for an existing execution. The response MUST include every persisted step record for that execution in ascending `sequence` order, including currently running action or wait steps. Each step item MUST include `sequence`, `step_name`, `status`, `started_at`, `ended_at` when present, `host` when present, `retry_count`, `exit_code` when present, `error_class` when present, `error_summary` when present, `command_id` when present, and any recorded stdout, stderr, or snapshot paths.

#### Scenario: Active execution exposes prior and current persisted steps

- **WHEN** client sends `GET /v1/switches/:id/steps` for an execution that has already persisted completed steps and is currently waiting in `awaiting_source_allocation` or `rebooting`
- **THEN** the system returns HTTP 200 with the prior completed steps followed by the current running wait step
- **AND** the running step omits `ended_at` until that wait finishes

#### Scenario: Failed precheck step exposes rejection reason

- **WHEN** client sends `GET /v1/switches/:id/steps` for an execution whose persisted timeline includes a failed precheck step classified as `precheck_blocked`
- **THEN** the failed step item includes `error_class=precheck_blocked`
- **AND** it includes `error_summary` with the operator-visible rejection reason when that summary was persisted

#### Scenario: Completed execution exposes full ordered history

- **WHEN** client sends `GET /v1/switches/:id/steps` for a completed, failed, or cancelled execution with persisted runtime history
- **THEN** the system returns HTTP 200 with all of that execution's step records ordered by ascending `sequence`
- **AND** the response preserves failure, rejection-summary, and evidence metadata recorded on individual steps

#### Scenario: Query step timeline for unknown execution

- **WHEN** client sends `GET /v1/switches/:id/steps` for a non-existent execution ID
- **THEN** the system returns HTTP 404 with an error indicating that the execution was not found

### Requirement: Query execution list with requested-time filters and pagination

The system SHALL return a list of executions via `GET /v1/switches` with optional query filters for `node`, `status`, `direction`, `requested_from`, and `requested_to`. The endpoint MUST also support history pagination with `limit` and `before` query parameters, where `before` is an RFC3339 timestamp used to request older executions inside the already-filtered result set. Results MUST be ordered by `requested_at` descending and each execution summary MUST include the fields needed by the dashboard history view, including `id`, `node_name`, `direction`, `current_state`, `overall_status`, `requested_at`, `requested_by`, and terminal error summary fields when present.

#### Scenario: List all executions without a requested-time range
- **WHEN** client sends GET `/v1/switches`
- **THEN** system returns HTTP 200 with an array of execution summaries ordered from newest to oldest

#### Scenario: Filter executions by requested-time range
- **WHEN** client sends GET `/v1/switches?requested_from=2026-06-11T00:00:00Z&requested_to=2026-06-18T23:59:59Z`
- **THEN** system returns HTTP 200 with only executions whose `requested_at` falls inside that requested time range

#### Scenario: Combine requested-time range with other filters
- **WHEN** client sends GET `/v1/switches?node=gpu-01&status=failed&direction=openstack_to_slurm&requested_from=2026-06-11T00:00:00Z&requested_to=2026-06-18T23:59:59Z`
- **THEN** system returns HTTP 200 with only executions that satisfy all supplied filters

#### Scenario: Page through older history inside a requested-time range
- **WHEN** client sends GET `/v1/switches?limit=10&requested_from=2026-06-11T00:00:00Z&requested_to=2026-06-18T23:59:59Z&before=2026-06-17T09:00:00Z`
- **THEN** system returns at most 10 execution summaries requested before that cursor timestamp
- **AND** every returned execution still falls inside the requested time range

#### Scenario: Reject an invalid requested-time range
- **WHEN** client sends GET `/v1/switches?requested_from=2026-06-18T23:59:59Z&requested_to=2026-06-11T00:00:00Z`
- **THEN** system returns HTTP 400
- **AND** the error indicates that the requested time range is invalid
