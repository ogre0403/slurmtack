## Context

`slurmtack` already has the core switch workflow, execution persistence, step persistence, and a token-protected REST API for creating and tracking executions. The current browser surface is only a static validation page under nginx, and the current API surface is centered on execution records instead of operator inventory views.

The requested UI needs a different read model:

1. Operators think in terms of Slurm partitions and nodes, not execution IDs.
2. The node list must start from Slurm partition membership, then answer "is this node effectively in Slurm or OpenStack right now?"
3. The same screen must expose switch actions and execution history without forcing direct database access.

Current code already has enough primitives to support this design:

- Slurm execution mutations and node-state reads exist.
- OpenStack compute-service and instance checks exist per host.
- Executions and ordered step records already exist in SQLite.
- nginx already serves static assets and proxies `/v1/`.

What is missing is a backend aggregation layer and a UI that consumes it.

## Goals / Non-Goals

**Goals:**
- Provide a browser dashboard that groups nodes by Slurm partition and shows a clear ownership/status summary for each node.
- Reuse the existing switch mutation endpoints from the UI instead of inventing parallel browser-only mutation APIs.
- Add dashboard read APIs that aggregate Slurm, OpenStack, active execution, and recent execution history into UI-friendly payloads.
- Let operators inspect execution history and ordered step records from the browser.

**Non-Goals:**
- Rework the orchestration state machine or switch semantics.
- Introduce push updates, websockets, or server-sent events in the first version.
- Expose raw log file contents in the browser.
- Solve multi-cluster federation beyond the currently configured Slurm and OpenStack control planes.

## Decisions

### 1. Use a dashboard-specific inventory read model

Decision:
- Add `GET /v1/dashboard/inventory` as a read-model endpoint instead of forcing the UI to fan out across many low-level APIs.

Rationale:
- The UI needs a partition list, node membership, node ownership summary, active execution summary, and last execution summary in one refresh cycle.
- Existing APIs do not expose partition membership or per-node OpenStack enrichment.
- A single aggregated payload keeps the frontend simple and avoids browser-side reconciliation bugs.

Alternatives considered:
- Add many small endpoints and let the UI compose them. Rejected because the existing backend does not expose the required primitives, and browser-side joins would duplicate business rules.
- Store a separate inventory table in SQLite. Rejected for the first version because current data can be read from live Slurm/OpenStack plus existing execution records.

### 2. Treat Slurm partitions as the source of truth for which nodes appear

Decision:
- The backend inventory aggregator will list Slurm partitions and their node membership first, then enrich each discovered node with Slurm/OpenStack/slurmtack state.

Rationale:
- This matches the user requirement that the UI be centered on nodes listed by Slurm partition.
- It avoids inventing a parallel node registry that could drift from the scheduler.

Alternatives considered:
- Start from OpenStack hypervisors and try to map them back to Slurm. Rejected because the operator wants the Slurm partition view to drive the inventory.
- Maintain a static config file for node membership. Rejected because it adds another source of truth.

### 3. Return partitions and node summaries in the same inventory response

Decision:
- `GET /v1/dashboard/inventory` returns:
  - `generated_at`
  - `partitions[]` with partition names and member node names
  - `nodes[]` with one normalized record per node, including `partitions[]`

Rationale:
- The UI can render grouped-by-partition views while avoiding duplicated node payloads for nodes that belong to multiple partitions.

Response sketch:

```json
{
  "generated_at": "2026-06-05T12:00:00Z",
  "partitions": [
    { "name": "gpu-maint", "nodes": ["gpu-01", "gpu-02"] }
  ],
  "nodes": [
    {
      "node_name": "gpu-01",
      "partitions": ["gpu-maint", "all"],
      "owner": "slurm",
      "owner_source": "derived",
      "slurm": {
        "state": "IDLE",
        "gres": ["gpu:a100:8"],
        "running_jobs": []
      },
      "openstack": {
        "compute_service": {
          "enabled": false,
          "status": "disabled",
          "state": "down"
        },
        "instance_count": 0,
        "active_migration_count": 0
      },
      "switch": {
        "available_direction": "slurm_to_openstack",
        "active_execution_id": "",
        "active_state": ""
      },
      "last_execution": {
        "id": "exec-123",
        "direction": "openstack_to_slurm",
        "overall_status": "succeeded",
        "requested_at": "2026-06-05T09:01:00Z"
      }
    }
  ]
}
```

### 4. Derive owner classification from control-plane state, not only execution state

Decision:
- The inventory endpoint reports a normalized owner badge with values `slurm`, `openstack`, `switching`, `unknown`, or `conflict`.
- `switching` is driven by an active execution.
- Otherwise ownership is derived from Slurm attach-state classification plus OpenStack compute-service visibility.

Rationale:
- Operators need an at-a-glance answer even when no execution is active.
- Execution history alone cannot describe steady-state ownership.

Alternatives considered:
- Persist `observed_owner` in SQLite first. Rejected for this change because the required data already exists from live control planes and no new orchestration contract is needed yet.

### 5. Keep switch mutations on existing endpoints

Decision:
- The dashboard uses existing mutation endpoints:
  - `POST /v1/switches`
  - `POST /v1/switches/:id/cancel`

Rationale:
- Mutation semantics already exist and are covered by current specs and tests.
- The dashboard should be a new client of existing switch APIs, not a second control plane.

UI behavior:
- For `openstack_to_slurm`, the selected node card provides `node_name`.
- For `slurm_to_openstack`, the action is launched from the partition context and may include `slurm_partition` plus optional `slurm_constraint`, but not `node_name`.

### 6. Add explicit history and step-detail APIs for browser drilldown

Decision:
- Extend `GET /v1/switches` with pagination and dashboard filters.
- Extend `GET /v1/switches/:id` with the execution metadata already stored in SQLite.
- Add `GET /v1/switches/:id/steps` for ordered step records.

Rationale:
- The existing execution list and detail responses are too thin for a history table and detail drawer.
- Step records already exist in the store layer, so exposing them is lower risk than asking operators to inspect SQLite or logs directly.

### 7. Build the first UI as a static same-origin app

Decision:
- Keep the UI under nginx static assets and have it call proxied `/v1/...` endpoints on the same origin.

Rationale:
- This matches the existing deployment shape.
- No extra frontend build pipeline is required for a first version.
- Same-origin requests simplify token usage and deployment topology.

Illustrative layout:

```text
┌─────────────────────────────────────────────────────────────────┐
│ slurmtack dashboard                              health: ok     │
├───────────────────────┬─────────────────────────┬───────────────┤
│ partitions            │ nodes in selected view  │ history       │
│ - all                 │ ┌─────────────────────┐ │ recent execs  │
│ - gpu-maint           │ │ gpu-01   owner:OS  │ │ filters        │
│ - gpu-prod            │ │ active: switching   │ │ detail drawer  │
│                       │ │ [switch] [cancel]   │ │               │
│                       │ └─────────────────────┘ │               │
└───────────────────────┴─────────────────────────┴───────────────┘
```

## Risks / Trade-offs

- [Inventory fan-out can be slow] -> Fetching partition, node, and OpenStack data per refresh may be expensive. Mitigation: keep the first version polling-based, allow `?partition=` filtering, and parallelize per-node enrichment in the backend.
- [Owner derivation can be ambiguous] -> Some Slurm/OpenStack combinations may not map cleanly to a single owner. Mitigation: expose `owner_source` and allow `unknown` or `conflict` instead of forcing a false answer.
- [Partition membership duplicates nodes] -> A node can belong to more than one partition. Mitigation: return normalized `nodes[]` plus per-node `partitions[]`, and let the UI render group membership without duplicating canonical state.
- [Dashboard read models can drift from operator expectations] -> Aggregated fields may hide raw control-plane details. Mitigation: include enough raw subfields (`slurm.state`, `openstack.compute_service`, counts, active execution state) so the UI can show both summary and evidence.

## Migration Plan

1. Add backend read endpoints and DTOs without changing switch mutation semantics.
2. Add Slurm partition-list support and backend inventory aggregation.
3. Replace the minimal nginx root page with the dashboard shell while preserving proxied health visibility.
4. Verify that existing API clients can continue using `POST /v1/switches`, `GET /v1/switches`, `GET /v1/switches/:id`, and cancel endpoints.
5. Roll back by restoring the old static page and leaving the new read endpoints unused; no state migration is required.

## Open Questions

- Should the inventory endpoint include cached timestamps per control plane, or is a single `generated_at` enough for the first version?
- Should the history list expose cursor-based pagination only, or also accept a simple `limit`/`offset` mode for manual operator use?
- Should the UI allow launching `slurm_to_openstack` from a partition header only, or also from any node currently classified as `slurm`?
