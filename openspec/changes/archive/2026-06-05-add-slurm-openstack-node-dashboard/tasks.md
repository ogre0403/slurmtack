## 1. Backend Inventory APIs

- [x] 1.1 Extend the Slurm client with partition and partition-node listing support needed for dashboard inventory reads.
- [x] 1.2 Add an authenticated `GET /v1/dashboard/inventory` handler that aggregates Slurm partition membership, Slurm node state, OpenStack host state, active execution state, and last execution summary.
- [x] 1.3 Define inventory DTOs and owner-classification rules for `slurm`, `openstack`, `switching`, `conflict`, and `unknown`.

## 2. Execution History APIs

- [x] 2.1 Expand `GET /v1/switches` to support `direction`, `limit`, and `before` filters while preserving newest-first ordering.
- [x] 2.2 Expand `GET /v1/switches/:id` to return the stored execution metadata needed for dashboard drilldown.
- [x] 2.3 Add `GET /v1/switches/:id/steps` to expose ordered step records for the selected execution.

## 3. Dashboard UI

- [x] 3.1 Replace the nginx root validation page with a dashboard shell that includes health, partition navigation, node inventory, and history regions.
- [x] 3.2 Implement same-origin dashboard data loading for `/api/health`, `/v1/dashboard/inventory`, `/v1/switches`, `/v1/switches/:id`, and `/v1/switches/:id/steps`.
- [x] 3.3 Implement UI actions for `openstack_to_slurm`, `slurm_to_openstack`, and cancellation using the existing switch mutation endpoints.

## 4. Verification

- [x] 4.1 Add backend tests for inventory aggregation, extended execution history filters, and step timeline responses.
- [x] 4.2 Add UI validation coverage for dashboard rendering, health failure states, and switch action request payloads.
- [x] 4.3 Update operator-facing docs to describe the dashboard workflow and new read endpoints.
