## Why

The current `openstack_to_slurm` flow makes operators do a two-step handoff: the REST API creates an execution, then a separate RabbitMQ client must publish the selected node before the orchestrator can continue. That split is operationally awkward and duplicates decision-making between the API caller and the external MQ producer.

This change restores a single API-driven entry point for `openstack_to_slurm`: the caller provides `node_name` in the POST body, and the API persists the execution and immediately publishes the corresponding MQ node-selection signal for the orchestrator.

## What Changes

- **BREAKING** Change `openstack_to_slurm` request semantics so `POST /v1/switches` requires `node_name` again instead of rejecting it.
- Update switch submission so the API/service layer publishes the `execution.node_selected` MQ message after persisting an `openstack_to_slurm` execution, rather than requiring a separate manual RabbitMQ publish.
- Keep MQ as the orchestrator's intake mechanism for node binding so downstream execution admission, state transitions, and lease acquisition continue to flow through the existing event-driven path.
- Preserve the current `slurm_to_openstack` requested-event behavior and leave the post-binding `openstack_to_slurm` workflow unchanged.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `rest-api`: change `openstack_to_slurm` request validation and accepted payload so `node_name` is supplied in the API request and no longer rejected.
- `amqp-events`: change the source of `execution.node_selected` so it is emitted by the API/service flow immediately after persistence for `openstack_to_slurm` requests.
- `gpu-node-switch-allocation`: change `openstack_to_slurm` node binding expectations so the node-selection event is derived from the accepted API request instead of a separate manual MQ client action.

## Impact

- Affected code: `internal/api`, `internal/service`, `internal/mq`, startup wiring in `cmd/main.go`, and related tests.
- API impact: `openstack_to_slurm` callers must provide `node_name` in `POST /v1/switches`; they no longer need a second RabbitMQ publish step.
- Operational impact: the API process becomes responsible for publishing both `execution.requested` and `execution.node_selected` for the appropriate directions, while the orchestrator remains MQ-driven.
