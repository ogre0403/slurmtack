## MODIFIED Requirements

### Requirement: Request a node switch

The system SHALL accept a switch request via `POST /v1/switches` and return a 202 response with an execution ID and status URL. The request body MUST include `direction` and `requested_by`. For `slurm_to_openstack`, the request MAY include `slurm_constraint` and `slurm_partition`. For `openstack_to_slurm`, the request MUST NOT require `node_name`; instead the system MUST create the execution in `awaiting_target_node` with an empty `node_name`, and the node MUST be correlated later through MQ before any node-bound action begins.

#### Scenario: Successful slurm_to_openstack request with explicit partition
- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_constraint": "gpu-a100", "slurm_partition": "gpu-maint"}`
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`

#### Scenario: Successful slurm_to_openstack request without explicit partition
- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_constraint": "gpu-a100"}`
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`

#### Scenario: Successful openstack_to_slurm request without node name
- **WHEN** client sends `POST /v1/switches` with `{"direction": "openstack_to_slurm", "requested_by": "operator-1"}`
- **THEN** system returns HTTP 202 with body containing `execution_id` and `status_url`
- **AND** the persisted execution enters `awaiting_target_node` with an empty `node_name`

#### Scenario: Openstack_to_slurm request rejects node_name in request body
- **WHEN** client sends `POST /v1/switches` with `{"direction": "openstack_to_slurm", "requested_by": "operator-1", "node_name": "gpu-01"}`
- **THEN** system returns HTTP 400 with an error indicating that `node_name` must be delivered through MQ node selection

#### Scenario: Missing required field
- **WHEN** client sends `POST /v1/switches` without `direction`
- **THEN** system returns HTTP 400 with error message indicating missing field

#### Scenario: Invalid direction value
- **WHEN** client sends `POST /v1/switches` with `{"direction": "invalid"}`
- **THEN** system returns HTTP 400 with error message indicating invalid direction

### Requirement: Query execution status

The system SHALL return execution details via `GET /v1/switches/:id` including current state, direction, node name when bound, overall status, timestamps, and error information if failed.

#### Scenario: Query existing execution
- **WHEN** client sends GET `/v1/switches/:id` for an existing execution
- **THEN** system returns HTTP 200 with execution details including `id`, `node_name`, `direction`, `current_state`, `overall_status`, `requested_at`, `requested_by`

#### Scenario: Query openstack_to_slurm execution before node binding
- **WHEN** client sends GET `/v1/switches/:id` for an `openstack_to_slurm` execution still waiting for MQ node selection
- **THEN** system returns HTTP 200 with `current_state` set to `awaiting_target_node`
- **AND** `node_name` is empty

#### Scenario: Query non-existent execution
- **WHEN** client sends GET `/v1/switches/:id` for a non-existent ID
- **THEN** system returns HTTP 404 with error message