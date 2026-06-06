## Context

The current dashboard already exposes an execution history list on the right and a detail drawer backed by `GET /v1/switches`, `GET /v1/switches/:id`, and `GET /v1/switches/:id/steps`. That implementation is still history-oriented: it loads 20 rows at a time with a "Load more" pattern, treats the list as secondary to node cards, and only exposes cancel from node cards that happen to show an active execution.

This change makes the right-hand panel an execution-first operator workflow. The panel must include running and completed executions in one browseable list, let operators inspect the selected execution's current state, and expose cancel directly from active execution rows without introducing new backend endpoints.

## Goals / Non-Goals

**Goals:**
- Present the right-side panel as an execution list instead of a generic history feed.
- Show both active and historical executions in the list, with execution state visible before opening details.
- Support paginated browsing with a default page size of 10 executions.
- Let operators cancel eligible active executions from the execution list and refresh the selected execution state after the request is accepted.
- Reuse existing execution list, execution detail, step timeline, and cancel endpoints.

**Non-Goals:**
- Changing backend execution ordering semantics or replacing the existing cursor-style list API.
- Introducing bulk cancellation, auto-refresh streaming, or new server-pushed updates.
- Redesigning the partition and node inventory portions of the dashboard beyond wiring them to the revised execution panel behavior.

## Decisions

### Keep a single execution list and paginate it with API cursors

The dashboard will continue to source execution summaries from `GET /v1/switches`, but it will request `limit=10` by default and track page cursors client-side so the UI can offer page-based navigation. Filters such as node and status continue to map to API query parameters, and changing filters resets pagination to the first page.

This approach keeps the backend contract unchanged while satisfying the pagination requirement. An alternative was splitting the panel into separate "running" and "history" lists. That was rejected because it duplicates execution rows across queries, complicates pagination, and makes selection state harder to reason about.

### Make the execution row the primary interaction surface

Each row in the right panel will show execution identity and summary fields that let operators decide whether to drill in: direction, node name when known, requested time, current state, and overall status. Selecting a row loads the existing execution detail and step timeline endpoints, and the detail area will surface `current_state` near the top of the summary instead of requiring operators to infer progress from the step list alone.

This keeps the current detail drawer pattern while aligning the panel with an execution-centric workflow. An alternative was to replace the drawer with inline row expansion, but that would reduce usable space for longer timelines and add more DOM complexity in the constrained sidebar layout.

### Add inline cancel controls only for active execution rows

The right-side list will render a cancel button only when the execution summary indicates an active execution. Clicking cancel will call `POST /v1/switches/:id/cancel`, stop row-selection event propagation, and refresh both the current list page and the selected execution detail so the operator sees the accepted state transition.

Node-card cancellation can remain as a secondary shortcut, but the execution list becomes the primary place to manage running work. An alternative was moving cancellation exclusively into the detail drawer; that was rejected because it adds unnecessary clicks for the most time-sensitive operator action.

## Risks / Trade-offs

- [Cursor-based pages can shift when new executions arrive] -> Reset to page 1 after actions that create or cancel executions, and treat page navigation as a live view rather than a stable historical snapshot.
- [Inline cancel buttons can conflict with row click selection] -> Use a dedicated button element that stops propagation and confirms intent before issuing the API call.
- [Active executions may occupy part of a page that would otherwise show older history] -> Accept this trade-off because the requirement is to include running executions in the same execution-first view.

## Migration Plan

Update the static dashboard assets and dashboard UI tests together. No data migration or API versioning change is required because the design reuses existing endpoints and response shapes.

Rollback is limited to restoring the previous static assets if the revised execution panel causes operator usability issues.

## Open Questions

- None at proposal time. The existing API already exposes the fields required for the execution-centric list, detail view, and cancellation flow.
