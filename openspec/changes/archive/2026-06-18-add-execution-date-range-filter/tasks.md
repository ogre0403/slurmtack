## 1. Backend execution list filtering

- [x] 1.1 Introduce an execution-list filter type in the store layer and update the SQLite and memory store `ListExecutions` paths to support requested-time range filtering together with node, status, direction, limit, and before semantics.
- [x] 1.2 Update `GET /v1/switches` in `internal/api/handler_switch.go` to parse and validate `requested_from` and `requested_to`, pass the filter set through the store, and preserve newest-first execution summaries.
- [x] 1.3 Add or update store and API tests to cover valid requested-time filtering, invalid/inverted ranges, and pagination inside a filtered window.

## 2. Dashboard execution history range UI

- [x] 2.1 Add start-date and end-date controls to the EXECUTIONS history filters in `docker/nginx/html/index.html` and initialize them to the local date range from seven days before today through today.
- [x] 2.2 Update `docker/nginx/html/dashboard.js` so execution list loading, polling, and filter changes translate the selected dates into `requested_from` and `requested_to` query parameters and reset pagination when the range changes.
- [x] 2.3 Add or update dashboard UI tests to verify the default recent-window behavior and that applying a narrower date range refreshes the execution list from page 1.

## 3. Documentation

- [x] 3.1 Update execution-history documentation to describe the new `requested_from` and `requested_to` query parameters and the dashboard's default "today back through the prior seven days" filter window.
