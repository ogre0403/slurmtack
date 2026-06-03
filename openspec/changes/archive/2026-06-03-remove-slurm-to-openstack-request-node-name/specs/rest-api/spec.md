## MODIFIED Requirements

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
