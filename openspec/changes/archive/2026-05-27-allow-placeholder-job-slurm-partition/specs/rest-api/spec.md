## MODIFIED Requirements

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
- **THEN** system returns HTTP 202 with body containing `execution_id` and `status_url`

#### Scenario: Missing required field

- **WHEN** client sends `POST /v1/switches` without `direction`
- **THEN** system returns HTTP 400 with error message indicating missing field

#### Scenario: Invalid direction value

- **WHEN** client sends `POST /v1/switches` with `{"direction": "invalid"}`
- **THEN** system returns HTTP 400 with error message indicating invalid direction