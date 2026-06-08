## Context

The project currently defines its HTTP API through Gin route registration in `internal/api/server.go` and request/response structs in `internal/api`, but it does not generate a Swagger artifact from that source of truth. Contributors can read handler code and OpenSpec documents, yet they do not have a machine-readable OpenAPI-style output that downstream tooling or reviewers can diff after API-facing changes.

Because the repository already keeps human-readable docs in `docs/`, the missing piece is a reproducible generation workflow that starts from Go annotations rather than handwritten JSON or YAML. The design also needs to fit the current code layout: `cmd/main.go` is the natural place for top-level API metadata, while the individual handler methods in `internal/api` own route semantics, request binding, and response models.

## Goals / Non-Goals

**Goals:**
- Add a repeatable repository command that generates Swagger artifacts from Go annotations.
- Keep the generated output in a stable docs path under version control so API changes can be reviewed as code and artifact diffs together.
- Document the current public HTTP handlers with enough annotation coverage for `swag` to emit usable request/response schemas.
- Avoid changing HTTP runtime behavior, route wiring, or adding a served Swagger UI as part of this change.

**Non-Goals:**
- Serving the generated Swagger files from the daemon or embedding Swagger UI in Gin.
- Reworking the existing OpenSpec REST API requirements into OpenAPI by hand.
- Introducing CI enforcement for Swagger drift in this change.

## Decisions

### 1. Standardize on `make swagger` as the generation entrypoint

**Choice:** Add a dedicated `swagger` target to `Makefile` that invokes `swag` with the repository's chosen arguments instead of expecting contributors to remember the raw command line.

- **Rationale:** The user specifically wants a Make target, and keeping one canonical entrypoint avoids drift in output location, parser flags, and general-info selection across contributors.
- **Alternatives Considered:**
  - *Document the raw `swag init ...` command in README only*: Rejected because it still relies on every contributor to copy the exact flags correctly.
  - *Hide generation behind a custom script*: Rejected because the current repository already centralizes common developer operations in `Makefile`, so another wrapper layer adds little value.

### 2. Generate only file artifacts under `docs/swagger/`

**Choice:** Configure the generation step to emit `swagger.json` and `swagger.yaml` into `docs/swagger/`, and avoid generating a `docs.go` runtime package unless a later change needs in-process Swagger serving.

- **Rationale:** The user asked for Swagger files generated from code annotations. JSON/YAML artifacts satisfy that requirement while avoiding an extra Go package that would need to compile during `go test ./...` even though the application does not consume it.
- **Alternatives Considered:**
  - *Generate the default `docs.go` package as well*: Rejected because it introduces an otherwise-unused package and extra module coupling without any runtime consumer.
  - *Write the files somewhere outside `docs/`*: Rejected because `docs/` is already the repository's documentation home and gives a stable reviewable location.

### 3. Put API metadata on `cmd/main.go` and operation annotations on handler methods

**Choice:** Add top-level Swagger metadata comments near `main` and annotate each public handler method in `internal/api` that backs a registered HTTP route. Where handlers currently return anonymous `gin.H` objects or implicit arrays, introduce explicit response models only when needed to make the generated schema stable and readable.

- **Rationale:** This matches the existing ownership boundaries in the code: the entrypoint describes the API as a whole, while handler methods already own route paths, methods, request DTOs, and response DTOs. Explicit response models keep generated schemas deterministic instead of relying on loosely documented map shapes.
- **Alternatives Considered:**
  - *Centralize all annotations in `server.go`*: Rejected because it separates route documentation from the handler code that actually defines request and response behavior.
  - *Hand-author Swagger YAML and treat code as secondary*: Rejected because the requested workflow is annotation-driven generation from code.

## Risks / Trade-offs

- **[Risk] Generated Swagger can drift if contributors forget to rerun `make swagger` after handler changes** → Mitigation: commit the generated artifacts and document `make swagger` as the canonical regeneration step in the repository workflow.
- **[Risk] Some handlers return shapes that are easy to serve but awkward to document** → Mitigation: add small explicit response structs where `swag` would otherwise infer weak schemas from `gin.H` or similar anonymous values.
- **[Risk] The generated spec may document conditionally mounted endpoints such as dashboard inventory** → Mitigation: keep the annotations aligned with the routes the server can register and note conditional availability in the endpoint descriptions where needed.

## Migration Plan

1. Add the `swagger` Make target and choose the exact `swag` output location and flags.
2. Annotate `cmd/main.go` and the HTTP handlers in `internal/api` so `swag` can discover API metadata, paths, and request/response models.
3. Generate `docs/swagger/swagger.json` and `docs/swagger/swagger.yaml`, then review the output to confirm it matches the registered routes.
4. Roll back by removing the annotations, generated artifacts, and Make target if the repository decides not to keep the generated docs.

## Open Questions

None.
