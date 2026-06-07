## MODIFIED Requirements

### Requirement: Request a node switch

The system SHALL accept a switch request via `POST /v1/switches` and return a 202 response with an execution ID and status URL. The request body MUST include `direction` and `requested_by`. For `slurm_to_openstack`, the request MAY include `slurm_constraint`, `slurm_partition`, and `slurm_account`; it MAY include `slurm_user` and `slurm_user_token` only as a complete override pair; and it MUST NOT include `node_name`. When `slurm_user` and `slurm_user_token` are both absent, the system MUST fall back to the configured daemon workload identity for Slurm. If neither the request nor the daemon configuration provides a complete workload identity, the system MUST reject the request before creating an execution. For `openstack_to_slurm`, the request MUST include `node_name`; before persisting an execution, the system MUST inspect the target node's current Slurm state and reject the request when that state already indicates active Slurm ownership. Accepted `openstack_to_slurm` requests MUST persist the execution in `awaiting_target_node` and use the supplied node to publish the MQ node-selection signal that continues the workflow.

#### Scenario: Successful slurm_to_openstack request with explicit partition and account

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_constraint": "gpu-a100", "slurm_partition": "gpu-maint", "slurm_account": "proj-123"}`
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`

#### Scenario: Successful slurm_to_openstack request with request-scoped workload credentials

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_user": "alice", "slurm_user_token": "jwt-123"}`
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`

#### Scenario: Successful slurm_to_openstack request without explicit partition or override credentials

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_constraint": "gpu-a100"}`
- **AND** the daemon is configured with a default workload Slurm identity
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`

#### Scenario: Slurm_to_openstack request rejects request-time node_name

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "node_name": "gpu-01"}`
- **THEN** system returns HTTP 400 with an error indicating that `node_name` is not accepted for `slurm_to_openstack`

#### Scenario: Slurm_to_openstack request rejects incomplete credential override

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_user": "alice"}`
- **THEN** system returns HTTP 400 with an error indicating that `slurm_user` and `slurm_user_token` must be provided together

#### Scenario: Slurm_to_openstack request rejects missing effective workload identity

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1"}`
- **AND** the daemon has no configured default workload Slurm identity
- **THEN** system returns HTTP 400 with an error indicating that a Slurm workload user and token are required
- **AND** the system does not create a new execution for that request

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
- **AND** Slurm reports `gpu-01` in an active schedulable state such as `idle`, `alloc`, or `mixed` with no drain or down token present
- **THEN** system returns a client-visible rejection indicating that `gpu-01` is already under Slurm ownership
- **AND** the system does not create a new execution for that request

#### Scenario: Missing required field

- **WHEN** client sends `POST /v1/switches` without `direction`
- **THEN** system returns HTTP 400 with error message indicating missing field

#### Scenario: Invalid direction value

- **WHEN** client sends `POST /v1/switches` with `{"direction": "invalid"}`
- **THEN** system returns HTTP 400 with error message indicating invalid direction

### Requirement: Query execution status

The system SHALL return execution details via `GET /v1/switches/:id` including current state, direction, node name when bound, overall status, timestamps, and error information if failed. The response MUST also include the execution metadata already persisted by the daemon that is required for operator drilldown, including `state_version`, `desired_owner`, `previous_owner`, lock timing, requested Slurm constraint, requested Slurm partition, requested Slurm account when present, placeholder job identifier when present, allocation event timestamp when present, and `cancellation_source_state` when cancellation was claimed. The response MUST NOT expose stored Slurm workload tokens or request-scoped Slurm workload usernames.

#### Scenario: Query existing execution with requested Slurm account

- **WHEN** client sends GET `/v1/switches/:id` for an existing `slurm_to_openstack` execution created with `slurm_account` set to `proj-123`
- **THEN** system returns HTTP 200 with execution details including `id`, `node_name`, `direction`, `current_state`, `overall_status`, `requested_at`, `requested_by`, and the persisted operator drilldown metadata
- **AND** the response includes `requested_slurm_account` with value `proj-123`
- **AND** the response does not include any persisted Slurm workload token or workload username field

#### Scenario: Query openstack_to_slurm execution before node binding consumption

- **WHEN** client sends GET `/v1/switches/:id` for an `openstack_to_slurm` execution that has been accepted but not yet advanced by the MQ consumer
- **THEN** system returns HTTP 200 with `current_state` set to `awaiting_target_node`
- **AND** `node_name` contains the API-supplied target node

#### Scenario: Query non-existent execution

- **WHEN** client sends GET `/v1/switches/:id` for a non-existent ID
- **THEN** system returns HTTP 404 with error message
