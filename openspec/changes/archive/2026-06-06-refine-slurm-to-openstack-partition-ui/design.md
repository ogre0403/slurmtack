## Context

The dashboard currently keeps both switch directions inside `buildNodeActions()` in `docker/nginx/html/dashboard.js`. `openstack_to_slurm` is correct there because the API requires `node_name`, but `slurm_to_openstack` is misleading in the same location because the workflow does not accept request-time node targeting. The current `switchFromPartition()` helper already omits `node_name` and only adds `slurm_partition` when `state.selectedPartition` is set, but the node-card button label and confirmation text still make the action look node-scoped.

The requested change is therefore mostly a dashboard interaction cleanup rather than a backend contract change. Existing API and allocation behavior already support both cases that matter here:
- omit `slurm_partition` to let Slurm use its default partition
- send `slurm_partition=<name>` to constrain placeholder allocation to a selected partition

## Goals / Non-Goals

**Goals:**
- Make the `slurm_to_openstack` entry point visually partition-scoped instead of node-scoped.
- Preserve the existing request payload contract: omit `slurm_partition` for `All`, include it for a specific partition.
- Keep `openstack_to_slurm` as a node-card action because that workflow still requires `node_name`.
- Update browser-facing tests and docs so the UI contract is explicit.

**Non-Goals:**
- Change the REST API request schema or daemon-side switch orchestration.
- Change `openstack_to_slurm` placement or semantics.
- Add partition validation, default-partition discovery, or any new backend endpoint.

## Decisions

### 1. Render `slurm_to_openstack` in a partition action bar above the node grid

Decision:
- Add a partition-context action area in the nodes panel header that shows the current partition selection and renders the `slurm_to_openstack` action there when it is valid to offer.

Rationale:
- The action depends on the selected partition, not on an individual node.
- Keeping it near the node grid preserves visibility without embedding an action into each partition list row.
- A dedicated action bar allows copy such as `Switch from All partitions to OpenStack` or `Switch from partition gpu-maint to OpenStack`, which matches the actual API payload.

Alternatives considered:
- Put the button directly inside the left partition list. Rejected because the list is primarily a navigation control and mixing selection with mutation makes the hit target and visual hierarchy harder to read.
- Keep the button on each node card and only change its label. Rejected because the placement still implies unsupported node targeting.

### 2. Treat `All` as the absence of a partition selector

Decision:
- Keep `state.selectedPartition === null` as the UI representation for `All`, and continue to build the `slurm_to_openstack` request body without `slurm_partition` in that state.

Rationale:
- The current frontend state model already represents `All` correctly.
- Omitting the field matches the existing API and allocation contracts for using the default partition.

Alternatives considered:
- Send `slurm_partition: "All"` or another sentinel value. Rejected because the backend contract expects omission, not a pseudo-partition name.
- Normalize `All` into a configured default partition client-side. Rejected because the dashboard should not guess cluster policy that the backend already handles by omission.

### 3. Remove node-scoped copy and callbacks for `slurm_to_openstack`

Decision:
- Update `buildNodeActions()` so node cards no longer render a `slurm_to_openstack` button, and replace the current `switchFromPartition(nodeName)` helper with a partition-scoped action that does not accept a node argument.

Rationale:
- Passing `nodeName` into the current helper is unused in the request body and only leaks into confirmation text, which reinforces the wrong mental model.
- A parameterless partition-scoped action is a simpler and more truthful interface.

Alternatives considered:
- Keep the unused node argument for convenience. Rejected because it preserves misleading call sites and weakens future tests.

### 4. Keep cancellation and `openstack_to_slurm` behavior unchanged

Decision:
- Leave node-card cancel actions and node-card `openstack_to_slurm` actions as they are.

Rationale:
- Those actions remain genuinely node-bound and are already aligned with the API contract.
- This keeps the change focused and lowers regression risk in the dashboard.

## Risks / Trade-offs

- [Partition-level action may feel less discoverable than a button on every Slurm-owned node] -> Add a visible action bar tied to the current partition selection instead of hiding the control inside the left navigation list.
- [Operators may expect the partition-level action to act on every visible node] -> Use explicit copy in the button and confirmation dialog that describes starting one `slurm_to_openstack` workflow scoped by partition selection, not bulk-switching all nodes.
- [Frontend tests currently assert only payload fragments, not button placement] -> Update tests to assert the new action container and the absence of node-scoped `slurm_to_openstack` wiring.

## Migration Plan

1. Update the dashboard HTML to include a partition action container near the node grid heading.
2. Update dashboard JS to render `slurm_to_openstack` from partition context, omit the node argument, and keep `All` mapped to no `slurm_partition`.
3. Update dashboard UI tests and operator docs to match the new placement and payload rules.
4. Roll back by restoring the previous node-card action wiring if operators reject the new interaction; no backend or data rollback is required.

## Open Questions

None.
