## 1. Runtime Step Persistence

- [x] 1.1 Add a shared step-lifecycle helper and stable workflow step-name constants for persisted execution history using the existing `steps` store model.
- [x] 1.2 Wire the orchestrator action path to create and complete persisted steps for runtime actions such as placeholder submission, lease acquisition, precheck, quiesce, reconfigure, reboot, attach, verify, completion, and cancel cleanup.
- [x] 1.3 Wire MQ intake, wait-exit, cancellation, and terminal failure paths to close or reuse running wait/action steps so active and aborted executions keep a coherent timeline.

## 2. Step Timeline API and Documentation

- [x] 2.1 Keep `GET /v1/switches/:id/steps` backed by the persisted runtime timeline and add handler/service coverage for active, completed, failed, and unknown execution queries.
- [x] 2.2 Update Swagger and operator-facing docs to describe the durable step timeline semantics and the detailed step metadata fields returned by the API.

## 3. Dashboard Execution Detail

- [x] 3.1 Expand the execution detail drawer in `docker/nginx/html/dashboard.js` to render richer step rows/cards with sequence, status, timing, host, retry, exit/error, and optional output/snapshot metadata.
- [x] 3.2 Add user-friendly label mapping and distinct presentation for running wait steps versus succeeded or failed action steps while preserving the existing execution selection, pagination, and cancel flows.

## 4. Verification

- [x] 4.1 Add or update orchestrator, MQ consumer, and integration tests proving deployed-style executions persist non-empty step timelines across action, wait, success, failure, and cancellation flows.
- [x] 4.2 Add or update API and dashboard UI tests covering detailed step rendering and non-empty `/v1/switches/:id/steps` responses for selected executions.
- [x] 4.3 Run the relevant Go test suites and dashboard/UI checks, then confirm the implemented behavior matches the new observability, API, and dashboard specs.
