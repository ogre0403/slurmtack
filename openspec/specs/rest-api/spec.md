## Purpose

Define REST API requirements for creating, tracking, listing, and authenticating GPU node switch executions.
## Requirements
### Requirement: Request a node switch

The system SHALL accept a switch request via `POST /v1/switches` and return a 202 response with an execution ID and status URL. The request body MUST include `direction` and `requested_by`. For `slurm_to_openstack`, the request MAY include `slurm_constraint` and `slurm_partition` and MUST NOT include `node_name`. For `openstack_to_slurm`, the request MUST include `node_name`; before persisting an execution, the system MUST inspect the target node's current Slurm state and reject the request when that state already indicates active Slurm ownership. Accepted `openstack_to_slurm` requests MUST persist the execution in `awaiting_target_node` and use the supplied node to publish the MQ node-selection signal that continues the workflow.

#### Scenario: Successful slurm_to_openstack request with explicit partition

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_constraint": "gpu-a100", "slurm_partition": "gpu-maint"}`
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`

#### Scenario: Successful slurm_to_openstack request without explicit partition

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_constraint": "gpu-a100"}`
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`

#### Scenario: Slurm_to_openstack request rejects request-time node_name

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "node_name": "gpu-01"}`
- **THEN** system returns HTTP 400 with an error indicating that `node_name` is not accepted for `slurm_to_openstack`

#### Scenario: Successful openstack_to_slurm request with node name

- **WHEN** client sends `POST /v1/switches` with `{"direction": "openstack_to_slurm", "requested_by": "operator-1", "node_name": "gpu-01"}`
- **AND** the current Slurm state for `gpu-01` is not already active Slurm ownership
- **THEN** system returns HTTP 202 with body containing `execution_id` and `status_url`
- **AND** the persisted execution enters `awaiting_target_node` before MQ consumption binds the node

#### Scenario: Openstack_to_slurm request rejects missing node_name

- **WHEN** client sends `POST /v1/switches` with `{"direction": "openstack_to_slurm", "requested_by": "operator-1"}`
- **THEN** system returns HTTP 400 with an error indicating that `node_name` is required for `openstack_to_slurm`

#### Scenario: Openstack_to_slurm request rejects node already owned by Slurm

- **WHEN** client sends `POST /v1/switches` with `{"direction": "openstack_to_slurm", "requested_by": "operator-1", "node_name": "gpu-01"}`
- **AND** Slurm reports `gpu-01` in an active schedulable state such as `idle`, `alloc`, or `mixed` with no drain/down token present
- **THEN** system returns a client-visible rejection indicating that `gpu-01` is already under Slurm ownership
- **AND** the system does not create a new execution for that request

#### Scenario: Missing required field

- **WHEN** client sends `POST /v1/switches` without `direction`
- **THEN** system returns HTTP 400 with error message indicating missing field

#### Scenario: Invalid direction value

- **WHEN** client sends `POST /v1/switches` with `{"direction": "invalid"}`
- **THEN** system returns HTTP 400 with error message indicating invalid direction

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

### Requirement: Token authentication

The system SHALL require a valid bearer token in the `Authorization` header for all `/v1/` endpoints. The token MUST match the configured `API_TOKEN` environment variable.

#### Scenario: Valid token

- **WHEN** client sends request with header `Authorization: Bearer <valid-token>`
- **THEN** system processes the request normally

#### Scenario: Missing token

- **WHEN** client sends request without Authorization header
- **THEN** system returns HTTP 401 with error message

#### Scenario: Invalid token

- **WHEN** client sends request with header `Authorization: Bearer wrong-token`
- **THEN** system returns HTTP 401 with error message

### Requirement: Health endpoint

The system SHALL expose `GET /health` without authentication that returns HTTP 200 when the daemon is running and the store is reachable.

#### Scenario: Healthy daemon

- **WHEN** client sends GET `/health`
- **THEN** system returns HTTP 200 with `{"status": "ok"}`

#### Scenario: Store unreachable

- **WHEN** the SQLite database is inaccessible
- **THEN** system returns HTTP 503 with `{"status": "unhealthy", "error": "<reason>"}`

### Requirement: Cancel stub endpoint

The system SHALL expose `POST /v1/switches/:id/cancel` to request operator cancellation of an existing execution. If the execution exists and is in `awaiting_target_node`, `awaiting_source_allocation`, or `source_quiescing`, the system MUST claim cancellation and return HTTP 202 with `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`. If the execution is already in `cancelling` or `cancelled`, the endpoint MUST behave idempotently and return the same HTTP 202 response shape. If the execution does not exist, the system MUST return HTTP 404. If the execution is in any other active state or is already terminal in another outcome, the system MUST return HTTP 409 with an error describing that the current state is not cancellable.

#### Scenario: Cancel waiting slurm_to_openstack execution

- **WHEN** client sends POST `/v1/switches/:id/cancel` for an existing `slurm_to_openstack` execution in `awaiting_source_allocation`
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`

#### Scenario: Cancel waiting openstack_to_slurm execution

- **WHEN** client sends POST `/v1/switches/:id/cancel` for an existing `openstack_to_slurm` execution in `source_quiescing`
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`

#### Scenario: Repeat cancel is idempotent

- **WHEN** client sends POST `/v1/switches/:id/cancel` for an execution already in `cancelling`
- **THEN** system returns HTTP 202 with body containing the same `execution_id` and `status_url`

#### Scenario: Cancel rejected for non-cancellable state

- **WHEN** client sends POST `/v1/switches/:id/cancel` for an execution in `rebooting`
- **THEN** system returns HTTP 409 with an error indicating that `rebooting` is not cancellable

#### Scenario: Cancel missing execution

- **WHEN** client sends POST `/v1/switches/:id/cancel` for a non-existent execution ID
- **THEN** system returns HTTP 404 with an error indicating that the execution was not found

### Requirement: Structured API access logs

The system SHALL emit one structured `log/slog` access log entry for every completed HTTP request handled by the daemon API, including unauthenticated `/health` requests and authenticated `/v1/` requests that succeed or fail authorization. Each access log entry MUST include the request method, the matched route pattern when available, the raw request path, the final HTTP status code, and request latency. The access log MUST NOT include bearer tokens or request bodies.

#### Scenario: Successful authenticated request is logged

- **WHEN** client sends an authenticated API request such as `GET /v1/switches/1234`
- **THEN** the daemon emits a structured access log entry for that request
- **AND** the entry includes `method`, route pattern `/v1/switches/:id`, raw path `/v1/switches/1234`, `status_code`, and latency fields

#### Scenario: Authorization failure is logged without token leakage

- **WHEN** client sends `POST /v1/switches` with a missing or invalid `Authorization` header
- **THEN** the daemon emits a structured access log entry with HTTP 401 for that request
- **AND** the log does not include the bearer token value or request body content

#### Scenario: Health endpoint is logged

- **WHEN** client sends `GET /health`
- **THEN** the daemon emits a structured access log entry for `/health` with the final HTTP status and latency
