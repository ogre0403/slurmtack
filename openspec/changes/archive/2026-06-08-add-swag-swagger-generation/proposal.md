## Why

The repository exposes multiple HTTP endpoints under `/health` and `/v1/*`, but it does not maintain a generated Swagger artifact that reflects the handlers implemented in code. That leaves API consumers and maintainers without a stable machine-readable contract and makes endpoint changes harder to review or publish.

## What Changes

- Add a repository-level Swagger generation workflow based on `swag`, with annotations living alongside the Go entrypoint and HTTP handlers that define the API surface.
- Generate Swagger output files into a committed docs location so the repository contains an up-to-date machine-readable API description derived from code.
- Add `Makefile` targets that run the Swagger generation workflow without requiring contributors to remember the raw `swag` command.
- Keep the existing HTTP routes and request/response behavior unchanged; this change adds documentation generation and contributor workflow support.

## Capabilities

### New Capabilities
- `swagger-generation`: generate Swagger documentation from in-code annotations through a standard repository command.

### Modified Capabilities
- None.

## Impact

- Affected code: `cmd/main.go`, `internal/api/*.go`, generated Swagger output under `docs/`, and `Makefile`.
- Affected dependencies: the repository will rely on `swag` for generation and may need Go module references required by the generated docs package.
- Affected workflow: contributors gain a single make target to regenerate Swagger artifacts after changing annotated API handlers.
