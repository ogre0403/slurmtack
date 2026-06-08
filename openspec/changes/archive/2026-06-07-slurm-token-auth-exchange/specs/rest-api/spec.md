## ADDED Requirements

### Requirement: Slurm Token Exchange
The system SHALL expose a `POST /v1/auth/login` endpoint that accepts a `slurm_user` and `slurm_user_token`. The system MUST validate the provided `slurm_user_token` against the configured Slurm REST API. To prevent credential spoofing, the system MUST decode the Slurm JWT locally and verify that the `sub` claim (or equivalent username claim) matches the provided `slurm_user`. If the token is valid and matches the user, the system SHALL return a short-lived slurmtack-signed Web Session JWT expiring in 1 hour.

#### Scenario: Successful Slurm Token Exchange
- **WHEN** client sends `POST /v1/auth/login` with `{"slurm_user": "alice", "slurm_user_token": "<valid-slurm-token>"}`
- **AND** the token validates successfully against the Slurm REST API and its `sub` claim matches `alice`
- **THEN** the system returns HTTP 200 with body `{"slurmtack_token": "<signed-jwt>"}`

#### Scenario: Rejected Slurm Token Exchange due to validation failure
- **WHEN** client sends `POST /v1/auth/login` with `{"slurm_user": "alice", "slurm_user_token": "<invalid-token>"}`
- **AND** the Slurm REST API returns an error or unauthorized status
- **THEN** the system returns HTTP 401 with body `{"error": "invalid slurm token"}`

#### Scenario: Rejected Slurm Token Exchange due to username mismatch
- **WHEN** client sends `POST /v1/auth/login` with `{"slurm_user": "bob", "slurm_user_token": "<valid-token-for-alice>"}`
- **THEN** the system returns HTTP 401 with body `{"error": "username mismatch"}`

### Requirement: Web Session Token Authentication
The system SHALL protect all `/v1/*` endpoints (except `/health` and `/v1/auth/login`) using the dynamic Web Session JWT. The authentication middleware MUST verify the signature, validity, and expiration of the JWT. If the token is valid, the middleware MUST extract the `sub` claim and bind it to the request context as the authenticated user.

#### Scenario: Authenticated request with valid Web Session JWT
- **WHEN** client sends a request with header `Authorization: Bearer <valid-web-jwt>`
- **THEN** the system authorizes the request and executes the handler with the authenticated user context bound

#### Scenario: Rejected request with expired Web Session JWT
- **WHEN** client sends a request with header `Authorization: Bearer <expired-web-jwt>`
- **THEN** the system rejects the request with HTTP 401 and body `{"error": "token expired"}`

## MODIFIED Requirements

### Requirement: Request a node switch

The system SHALL accept a switch request via `POST /v1/switches` and return a 202 response with an execution ID and status URL. The request body MUST include `direction` and `requested_by`. When the request is authenticated via a dynamic Web Session JWT, the system MUST verify that the `requested_by` field matches the authenticated username or override it with the authenticated username to ensure accountability. For `slurm_to_openstack`, the request MAY include `slurm_constraint`, `slurm_partition`, `slurm_account`, and `placeholder_sif_file`; it MAY include `slurm_user` and `slurm_user_token` only as a complete override pair; and it MUST NOT include `node_name`. When `slurm_user` and `slurm_user_token` are both absent, the system MUST fall back to the configured daemon workload identity for Slurm. If neither the request nor the daemon configuration provides a complete workload identity, the system MUST reject the request before creating an execution. For `slurm_to_openstack`, the system MUST also resolve one effective placeholder SIF filename before creating an execution: use `placeholder_sif_file` when provided, otherwise fall back to the configured daemon default filename. The effective filename MUST be a simple basename, and the request MUST be rejected when the filename is missing or invalid, or when the daemon cannot resolve a valid home-relative placeholder SIF directory configuration. Accepted `slurm_to_openstack` requests MUST persist the effective placeholder SIF filename needed for later asynchronous placeholder submission. For `openstack_to_slurm`, the request MUST include `node_name`; before persisting an execution, the system MUST inspect the target node's current Slurm state and reject the request when that state already indicates active Slurm ownership. Accepted `openstack_to_slurm` requests MUST persist the execution in `awaiting_target_node` and use the supplied node to publish the MQ node-selection signal that continues the workflow.

#### Scenario: Successful slurm_to_openstack request with verified user binding
- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "alice"}`
- **AND** the request is authenticated via Web Session JWT with `sub=alice`
- **THEN** the system accepts the request and persists the execution with `requested_by=alice`

#### Scenario: Rejected slurm_to_openstack request due to spoofed requested_by
- **WHEN** client sends `POST /v1/switches` with `{"direction": "slurm_to_openstack", "requested_by": "bob"}`
- **AND** the request is authenticated via Web Session JWT with `sub=alice`
- **THEN** the system rejects the request with HTTP 400 or overrides `requested_by` with `alice` to ensure accountability
