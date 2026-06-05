## Why

Operators currently have to combine `/v1/switches`, daemon logs, Slurm queries, and OpenStack queries by hand to understand which GPU nodes belong to Slurm versus OpenStack. That is slow for day-to-day operations and makes it hard to safely trigger a switch or inspect previous executions from a browser.

## What Changes

- Add an operator-facing dashboard UI served by nginx that shows backend health, Slurm partitions, node ownership, active switch progress, and recent execution history.
- Add dashboard-oriented REST API endpoints that use Slurm partition membership as the source of truth for which nodes to display, then enrich each node with Slurm, OpenStack, and slurmtack execution state.
- Expand execution history APIs so the UI can filter, paginate, and inspect execution details and ordered step records without reading SQLite or logs directly.
- Keep `POST /v1/switches` and `POST /v1/switches/:id/cancel` as the mutation boundary, but define the UI behaviors and API payloads needed to drive those actions safely from the browser.

## Capabilities

### New Capabilities
- `node-switch-dashboard`: An operator dashboard for partition-scoped node status, switch actions, and execution history drilldown.

### Modified Capabilities
- `rest-api`: Add node inventory read models and richer execution history/detail endpoints for the dashboard.
- `health-check-ui`: Replace the minimal validation page at `/` with a dashboard shell that still surfaces proxied backend health.

## Impact

- Affected code: `internal/api`, `internal/slurm`, `internal/openstack`, nginx static assets under `docker/nginx/html`, and related tests.
- Affected APIs: new dashboard inventory and execution-step read endpoints; expanded execution list/detail responses.
- Dependencies: requires Slurm partition and node listing support plus OpenStack compute status lookups per Slurm-discovered node.
- Operators gain a browser-first workflow for monitoring ownership and initiating switches, while existing token-protected mutation APIs remain the authority.
