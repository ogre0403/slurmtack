## Why

The current dashboard still presents `slurm_to_openstack` as if the operator can target a node directly, even though that workflow only selects a node later through placeholder allocation. This causes a UI mismatch: the `PARTITION=All` case should use the cluster default partition by omitting `slurm_partition`, while a specific partition selection should scope the placeholder job to that partition.

## What Changes

- Adjust the dashboard's `slurm_to_openstack` action so `PARTITION=All` submits `POST /v1/switches` without `slurm_partition`.
- Adjust the dashboard's `slurm_to_openstack` action so selecting a concrete partition submits that partition and lets the placeholder job allocate within it.
- Move the `slurm_to_openstack` trigger out of the node-level action area to partition-level UI so the screen no longer implies request-time node targeting.
- Preserve the existing `openstack_to_slurm` node-level interaction because that workflow still requires an explicit `node_name`.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `node-switch-dashboard`: Change the dashboard contract for how partition context drives `slurm_to_openstack` requests and where the action is rendered.

## Impact

- Frontend dashboard layout and request-building logic for switch actions
- Browser-side tests covering partition filtering and switch submission payloads
- Operator-facing workflow clarity for `slurm_to_openstack` versus `openstack_to_slurm`
