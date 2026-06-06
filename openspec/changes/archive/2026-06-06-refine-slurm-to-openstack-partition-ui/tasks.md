## 1. Partition-scoped dashboard UI

- [x] 1.1 Update `docker/nginx/html/index.html` to add a partition action area near the node grid that can host the `slurm_to_openstack` control outside node cards.
- [x] 1.2 Update `docker/nginx/html/dashboard.js` so node cards keep `openstack_to_slurm` and cancel actions but no longer render a `slurm_to_openstack` button.
- [x] 1.3 Render the `slurm_to_openstack` control from partition context and label it with the current selection so `All` and specific partitions are clearly distinguished.

## 2. Switch request behavior

- [x] 2.1 Refactor the partition-scoped `slurm_to_openstack` action helper to stop accepting a node argument and to use partition-focused confirmation copy.
- [x] 2.2 Ensure the `slurm_to_openstack` request body omits `slurm_partition` when `All` is selected and includes `slurm_partition=<name>` when a specific partition is selected.
- [x] 2.3 Verify the refreshed inventory and history flows still run after successful partition-scoped switch creation.

## 3. Verification and documentation

- [x] 3.1 Update `internal/api/dashboard_ui_test.go` to assert the new action container, the absence of node-scoped `slurm_to_openstack` wiring, and the preserved `openstack_to_slurm` payload shape.
- [x] 3.2 Add or update frontend-facing assertions for the partition-scoped `slurm_to_openstack` payload so `All` omits `slurm_partition` and specific partitions include it.
- [x] 3.3 Update `docs/dashboard.md` to describe the new placement and semantics of the `slurm_to_openstack` control.
