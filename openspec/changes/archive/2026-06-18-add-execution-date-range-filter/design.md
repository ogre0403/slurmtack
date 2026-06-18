## Context

The dashboard execution panel currently exposes node, status, and direction filters, then calls `GET /v1/switches?limit=...` with those query parameters plus the existing `before` cursor for pagination. The backend list handler sorts all executions in memory and applies the non-node filters after reading them from the store, while the store interface itself only supports an optional node filter.

This change adds a requested-date window to the execution history. The operator expectation is calendar-based, not raw timestamp-based: the EXECUTIONS panel should open on "today back through the prior seven days" and continue to page, poll, and combine filters consistently inside that selected window.

## Goals / Non-Goals

**Goals:**
- Add start-date and end-date controls to the dashboard execution history panel.
- Default the dashboard execution list to the local calendar range from seven days before today through today.
- Extend `GET /v1/switches` so execution summaries can be filtered by requested time range in combination with node, status, direction, `limit`, and `before`.
- Keep newest-first ordering, existing detail drilldown, polling, and page navigation behavior intact inside the selected date range.

**Non-Goals:**
- Adding preset shortcuts such as "Last 24 hours" or "Last 30 days".
- Introducing a separate reporting endpoint or changing execution detail/step APIs.
- Adding per-user server-side persistence for the selected history window.

## Decisions

### Use explicit requested-time query parameters on `GET /v1/switches`

The API will add `requested_from` and `requested_to` query parameters for execution list queries. Both parameters will use RFC3339 timestamps so the backend remains timezone-explicit and can combine them with the existing `before` cursor without overloading cursor semantics.

The handler will validate malformed timestamps and reject a range where `requested_from` is after `requested_to`. An alternative was filtering only in the browser after fetching pages from the existing API. That was rejected because it would make pagination inaccurate, hide matching executions that are not on the current page, and increase polling waste as history grows.

### Treat the dashboard controls as local-date inputs and translate them into API bounds

The UI will expose two date inputs, defaulting to the operator's local `today` and the local date seven days earlier. When the dashboard queries the API, it will translate the selected start date to the beginning of that local day and the selected end date to the end of that local day, then send those RFC3339 bounds as `requested_from` and `requested_to`.

This preserves the operator's calendar mental model even though execution timestamps are stored as instants. An alternative was sending raw `YYYY-MM-DD` strings and letting the server guess the timezone. That was rejected because the daemon does not know the browser's timezone and would produce inconsistent boundaries for remote operators.

### Push filtering semantics into the store contract instead of leaving them in the handler

The current list path reads all executions from the store and applies most filters in the API handler. For date-range filtering, the store contract should evolve to accept a list-filter object that includes node, status, direction, requested-from, requested-to, limit, and before. SQLite can then apply the range in SQL, and the in-memory store can mirror the same behavior for tests.

This keeps the list API efficient and makes pagination operate on the already-filtered result set. An alternative was preserving the current store signature and continuing to filter everything in the handler. That was rejected because it would keep full-history reads on every poll and split filtering behavior across layers as the query contract grows.

## Risks / Trade-offs

- [Local-date defaults can surprise operators working across timezones] -> Define the dashboard behavior explicitly around the browser's local calendar date and translate to RFC3339 bounds before calling the API.
- [Adding another pair of query parameters increases list-handler validation complexity] -> Keep the validation narrow: parse both timestamps, reject invalid format, and reject inverted ranges with clear `HTTP 400` errors.
- [Changing the store list contract touches both SQLite and memory implementations] -> Use a dedicated filter struct so future list filters extend one type instead of repeatedly changing method signatures.

## Migration Plan

No schema migration is required because executions already persist `requested_at`. Deploy the API/store change and updated dashboard assets together so the new UI controls map to supported query parameters immediately.

Rollback is a normal code and static-asset rollback. If the new filter behavior causes operator confusion, the system can return to the prior list query path without data repair.

## Open Questions

- None at proposal time.
