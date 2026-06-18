## Why

The dashboard execution history currently lets operators filter by node, status, and direction, but it always queries the full history range. As execution volume grows, operators need a date window to focus on recent activity by default and narrow the list to the period they are investigating.

## What Changes

- Add a requested-date-range filter to the dashboard EXECUTIONS list using a start date and end date control.
- Default the dashboard execution history to executions requested from the current day back through the prior seven days.
- Extend `GET /v1/switches` so execution history queries can filter by requested time range alongside the existing node, status, direction, and pagination filters.
- Preserve newest-first ordering and existing pagination behavior while applying the requested date range.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `node-switch-dashboard`: the execution history panel adds a date-range filter, defaults to the recent seven-day window ending today, and refreshes the paginated list when the range changes.
- `rest-api`: `GET /v1/switches` accepts requested-time range filters so the dashboard can fetch only executions inside the selected window.

## Impact

- Affected UI: `docker/nginx/html/index.html` and `docker/nginx/html/dashboard.js` execution history controls and query construction.
- Affected API/store path: `internal/api/handler_switch.go`, `internal/store/store.go`, `internal/store/sqlite.go`, and `internal/store/memory.go`.
- Affected tests/docs: dashboard UI/history tests, store/API list tests, and execution-list API/dashboard documentation.
