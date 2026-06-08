## ADDED Requirements

### Requirement: Generate Swagger artifacts from code annotations through a repository command
The repository SHALL provide a `make swagger` workflow that runs `swag` against the Go API source and regenerates committed Swagger artifacts under `docs/swagger/`. The workflow MUST produce `swagger.json` and `swagger.yaml` from in-code annotations rather than maintaining handwritten Swagger files.

#### Scenario: Regenerate Swagger artifacts
- **WHEN** a contributor runs `make swagger` from the repository root
- **THEN** the command completes using the repository-defined `swag` arguments
- **AND** `docs/swagger/swagger.json` and `docs/swagger/swagger.yaml` are regenerated from the current code annotations

### Requirement: Annotate the public HTTP API surface used for Swagger generation
The Swagger generation inputs SHALL include top-level API metadata in `cmd/main.go` and operation annotations for the public HTTP handlers registered by `internal/api/server.go`. The documented operations MUST cover `/health`, `/v1/auth/login`, `/v1/switches`, `/v1/switches/{id}`, `/v1/switches/{id}/steps`, `/v1/switches/{id}/cancel`, `/v1/dashboard/settings`, and the conditionally mounted `/v1/dashboard/inventory` route, using explicit request and response models wherever anonymous response shapes would make the generated schema ambiguous.

#### Scenario: Generated spec lists the annotated routes
- **WHEN** the repository Swagger artifacts are regenerated from the annotated codebase
- **THEN** the resulting Swagger document contains operations for the annotated public routes registered by the API server
- **AND** each documented operation references stable request and response schemas derived from the annotated Go models
