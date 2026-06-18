## ADDED Requirements

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
