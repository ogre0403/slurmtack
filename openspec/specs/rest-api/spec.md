## ADDED Requirements

### Requirement: Request a node switch

The system SHALL accept a switch request via `POST /v1/switches` and return a 202 response with an execution ID and status URL. The request body MUST include `direction` and `requested_by`. It MAY include `node_name` (required for `openstack_to_slurm`), `slurm_constraint` (for `slurm_to_openstack`), and `slurm_partition` (for `slurm_to_openstack` placeholder selection).

#### Scenario: Successful slurm_to_openstack request with explicit partition

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_constraint": "gpu-a100", "slurm_partition": "gpu-maint"}`
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`

#### Scenario: Successful slurm_to_openstack request without explicit partition

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_constraint": "gpu-a100"}`
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`

#### Scenario: Successful openstack_to_slurm request

- **WHEN** client sends `POST /v1/switches` with `{"direction": "openstack_to_slurm", "requested_by": "operator-1", "node_name": "gpu-01"}`
- **THEN** system returns HTTP 202 with body containing execution_id and status_url

#### Scenario: Missing required field

- **WHEN** client sends `POST /v1/switches` without `direction`
- **THEN** system returns HTTP 400 with error message indicating missing field

#### Scenario: Invalid direction value

- **WHEN** client sends `POST /v1/switches` with `{"direction": "invalid"}`
- **THEN** system returns HTTP 400 with error message indicating invalid direction

### Requirement: Query execution status

The system SHALL return execution details via `GET /v1/switches/:id` including current state, direction, node name, overall status, timestamps, and error information if failed.

#### Scenario: Query existing execution

- **WHEN** client sends GET /v1/switches/:id for an existing execution
- **THEN** system returns HTTP 200 with execution details including `id`, `node_name`, `direction`, `current_state`, `overall_status`, `requested_at`, `requested_by`

#### Scenario: Query non-existent execution

- **WHEN** client sends GET /v1/switches/:id for a non-existent ID
- **THEN** system returns HTTP 404 with error message

### Requirement: List executions

The system SHALL return a list of executions via `GET /v1/switches` with optional query filters for `node` and `status`.

#### Scenario: List all executions

- **WHEN** client sends GET /v1/switches
- **THEN** system returns HTTP 200 with array of execution summaries

#### Scenario: Filter by node name

- **WHEN** client sends GET /v1/switches?node=gpu-01
- **THEN** system returns HTTP 200 with only executions matching node_name "gpu-01"

#### Scenario: Filter by overall status

- **WHEN** client sends GET /v1/switches?status=active
- **THEN** system returns HTTP 200 with only executions with overall_status "active"

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

- **WHEN** client sends GET /health
- **THEN** system returns HTTP 200 with `{"status": "ok"}`

#### Scenario: Store unreachable

- **WHEN** the SQLite database is inaccessible
- **THEN** system returns HTTP 503 with `{"status": "unhealthy", "error": "<reason>"}`

### Requirement: Cancel stub endpoint

The system SHALL expose `POST /v1/switches/:id/cancel` that returns HTTP 501 Not Implemented. This endpoint will be implemented in a future change.

#### Scenario: Cancel request

- **WHEN** client sends POST /v1/switches/:id/cancel
- **THEN** system returns HTTP 501 with `{"error": "not implemented"}`
