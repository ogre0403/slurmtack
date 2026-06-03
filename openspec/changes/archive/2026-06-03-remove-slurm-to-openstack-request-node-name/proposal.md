## Why

The `slurm_to_openstack` workflow does not know the target node at request time; the actual node is only determined later by the placeholder allocation event. Allowing `POST /v1/switches` to carry `node_name` for this direction creates a misleading pre-bound execution record and conflicting operator expectations about when node identity becomes authoritative.

## What Changes

- Reject or otherwise disallow `node_name` on `POST /v1/switches` when `direction=slurm_to_openstack`.
- Keep newly accepted `slurm_to_openstack` executions unbound until the placeholder agent publishes the allocation event.
- Update API responses, tests, and operator-facing docs so they no longer imply that `slurm_to_openstack` can be submitted with a selected node.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `rest-api`: Change the switch request contract so `slurm_to_openstack` does not accept or persist `node_name` before allocation.

## Impact

- Affects `internal/api`, `internal/service`, and related API/service tests for switch submission and status/list behavior.
- Affects operator documentation such as `README.md` and any request examples that still describe `node_name` as a valid `slurm_to_openstack` input.
- Preserves the existing MQ allocation/drained event contracts; node binding remains driven by placeholder-agent allocation.
