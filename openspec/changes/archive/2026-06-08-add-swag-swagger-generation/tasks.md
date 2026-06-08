## 1. Swagger generation workflow

- [x] 1.1 Add a `swagger` target to `Makefile` that runs the repository-standard `swag` command and regenerates `docs/swagger/swagger.json` plus `docs/swagger/swagger.yaml`.
- [x] 1.2 Create or normalize the repository output location and any tool invocation details needed so contributors can regenerate the Swagger artifacts from the repo root without ad-hoc command flags.

## 2. API annotation coverage

- [x] 2.1 Add top-level Swagger metadata in `cmd/main.go` and introduce any explicit request/response models needed to document currently anonymous payloads cleanly.
- [x] 2.2 Annotate the auth and switch handlers in `internal/api` so the generated Swagger describes login, switch creation, switch detail/history, step listing, and cancel operations.
- [x] 2.3 Annotate the health, dashboard settings, and dashboard inventory handlers so the generated Swagger covers the remaining public routes, including any conditional-availability notes needed for inventory.

## 3. Generated artifacts and workflow documentation

- [x] 3.1 Run `make swagger` and commit the regenerated `docs/swagger/swagger.json` and `docs/swagger/swagger.yaml` artifacts.
- [x] 3.2 Review the generated Swagger output to confirm all required public routes and stable schemas are present, then update contributor-facing docs if needed to point developers at `make swagger` as the canonical regeneration step.
