## MODIFIED Requirements

### Requirement: Annotate the public HTTP API surface used for Swagger generation

The Swagger generation inputs SHALL include top-level API metadata in `cmd/main.go` and operation annotations for the public HTTP handlers registered by `internal/api/server.go`. The documented operations MUST cover `/health`, `/v1/auth/login`, `/v1/switches`, `/v1/switches/{id}`, `/v1/switches/{id}/steps`, `/v1/switches/{id}/cancel`, and the conditionally mounted `/v1/dashboard/inventory` route, using explicit request and response models wherever anonymous response shapes would make the generated schema ambiguous. The generated descriptions for `/v1/dashboard/inventory` and `POST /v1/switches` MUST explain the optional `SLURM_CLOUD_PARTITION` scoped behavior when that deployment setting is configured.

#### Scenario: Generated spec lists the annotated routes

- **WHEN** the repository Swagger artifacts are regenerated from the annotated codebase
- **THEN** the resulting Swagger document contains operations for the annotated public routes registered by the API server
- **AND** each documented operation references stable request and response schemas derived from the annotated Go models
- **AND** the generated document does not include a removed `/v1/dashboard/settings` operation
- **AND** the generated descriptions for `/v1/dashboard/inventory` and `POST /v1/switches` mention the optional cloud-partition scope
