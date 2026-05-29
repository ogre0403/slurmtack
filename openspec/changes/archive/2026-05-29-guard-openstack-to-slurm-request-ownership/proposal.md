## Why

The current `openstack_to_slurm` request path accepts a node that is already effectively back in active Slurm service, so the daemon treats it as fresh work and reruns the full handoff workflow, including reboot. That is unsafe and operationally surprising because a request that should be rejected instead causes another disruptive mutation.

## What Changes

- **BREAKING** Change `openstack_to_slurm` request admission so `POST /v1/switches` rejects nodes that are already back in an active Slurm state instead of creating a new execution.
- Add an ownership guard in the API/service request path that checks the target node's current Slurm state before persisting an `openstack_to_slurm` execution.
- Define the request-time classification for "already owned by Slurm" so the guard blocks the duplicate handoff before MQ publish, lease acquisition, host mutation, or reboot can start.
- Preserve the existing workflow for nodes that are still genuinely under OpenStack ownership and still need to move into Slurm.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `rest-api`: change `openstack_to_slurm` admission so requests for nodes already owned by Slurm are rejected at `POST /v1/switches` instead of returning a new execution.
- `gpu-node-switch-orchestration`: change request-stage workflow requirements so duplicate OpenStack-to-Slurm ownership transitions are blocked before execution creation.

## Impact

- Affected code: `internal/api`, `internal/service`, `internal/slurm`, request-path wiring in `cmd/main.go`, and the related tests.
- API impact: `openstack_to_slurm` callers can receive a request-stage rejection when the target node is already considered Slurm-owned; no execution ID is created in that case.
- Operational impact: duplicate OpenStack-to-Slurm requests stop at admission time instead of re-entering the destructive switch workflow.
