## Why

The dashboard's right-side history needs to become an execution-focused operator workflow instead of a loosely defined history list. Operators need one place to see active executions, review prior executions, inspect current state, and cancel eligible in-flight work without leaving the dashboard.

## What Changes

- Rework the dashboard right-side history panel to list executions as the primary unit, including both currently active executions and prior executions.
- Make each execution row selectable so the detail area shows the execution's current state and related summary details.
- Expose a cancel action for executions that are still running and already supported by the existing cancellation API.
- Add paginated browsing for the execution list with a default page size of 10 items per page.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `node-switch-dashboard`: Change the dashboard history experience so the right-side panel is execution-centric, includes active and historical executions, supports detail drilldown on selection, exposes cancel for running executions, and paginates the list at 10 items per page by default.

## Impact

- Affects the browser dashboard UI at `/`, especially the right-side history/detail panel and its client-side state management.
- Reuses existing execution listing, execution detail, and cancellation APIs instead of introducing new endpoints.
- Requires dashboard tests to cover execution ordering, pagination, selection, and cancel affordances for active executions.
