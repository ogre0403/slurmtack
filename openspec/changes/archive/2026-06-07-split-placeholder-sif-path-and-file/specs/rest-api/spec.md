## MODIFIED Requirements

### Requirement: Request a node switch

The system SHALL accept a switch request via `POST /v1/switches` and return a 202 response with an execution ID and status URL. The request body MUST include `direction` and `requested_by`. For `slurm_to_openstack`, the request MAY include `slurm_constraint`, `slurm_partition`, `slurm_account`, and `placeholder_sif_file`; it MAY include `slurm_user` and `slurm_user_token` only as a complete override pair; and it MUST NOT include `node_name`. When `slurm_user` and `slurm_user_token` are both absent, the system MUST fall back to the configured daemon workload identity for Slurm. If neither the request nor the daemon configuration provides a complete workload identity, the system MUST reject the request before creating an execution. For `slurm_to_openstack`, the system MUST also resolve one effective placeholder SIF filename before creating an execution: use `placeholder_sif_file` when provided, otherwise fall back to the configured daemon default filename. The effective filename MUST be a simple basename, and the request MUST be rejected when the filename is missing or invalid, or when the daemon cannot resolve a valid home-relative placeholder SIF directory configuration. Accepted `slurm_to_openstack` requests MUST persist the effective placeholder SIF filename needed for later asynchronous placeholder submission. For `openstack_to_slurm`, the request MUST include `node_name`; before persisting an execution, the system MUST inspect the target node's current Slurm state and reject the request when that state already indicates active Slurm ownership. Accepted `openstack_to_slurm` requests MUST persist the execution in `awaiting_target_node` and use the supplied node to publish the MQ node-selection signal that continues the workflow.

#### Scenario: Successful slurm_to_openstack request with explicit partition, account, and placeholder SIF filename

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_constraint": "gpu-a100", "slurm_partition": "gpu-maint", "slurm_account": "proj-123", "placeholder_sif_file": "placeholder-agent-debug.sif"}`
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`
- **AND** the execution persists `placeholder-agent-debug.sif` as the effective placeholder SIF filename for later placeholder submission

#### Scenario: Successful slurm_to_openstack request with request-scoped workload credentials

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_user": "alice", "slurm_user_token": "jwt-123"}`
- **AND** the daemon is configured with a valid home-relative placeholder SIF path and default filename
- **THEN** system returns HTTP 202 with body `{"execution_id": "<id>", "status_url": "/v1/switches/<id>"}`

#### Scenario: Successful slurm_to_openstack request without explicit partition, override credentials, or filename override

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "slurm_constraint": "gpu-a100"}`
- **AND** the daemon is configured with a default workload Slurm identity and a default placeholder SIF filename
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

#### Scenario: Slurm_to_openstack request rejects invalid placeholder SIF filename

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "placeholder_sif_file": "../other-user/agent.sif"}`
- **THEN** system returns HTTP 400 with an error indicating that `placeholder_sif_file` must be a simple filename
- **AND** the system does not create a new execution for that request

#### Scenario: Slurm_to_openstack request rejects missing effective placeholder SIF filename

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1"}`
- **AND** the daemon has a valid home-relative placeholder SIF path but no configured default placeholder SIF filename
- **AND** the request does not provide `placeholder_sif_file`
- **THEN** system returns HTTP 400 with an error indicating that a placeholder SIF filename is required
- **AND** the system does not create a new execution for that request

#### Scenario: Slurm_to_openstack request rejects missing valid placeholder SIF path config

- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "operator-1", "placeholder_sif_file": "placeholder-agent.sif"}`
- **AND** the daemon does not have a valid home-relative `PLACEHOLDER_SIF_PATH` configuration
- **THEN** system returns HTTP 400 with an error indicating that placeholder SIF path configuration is invalid or missing
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
