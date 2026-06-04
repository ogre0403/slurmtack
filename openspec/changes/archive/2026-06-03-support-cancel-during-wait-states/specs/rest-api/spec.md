## MODIFIED Requirements

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
