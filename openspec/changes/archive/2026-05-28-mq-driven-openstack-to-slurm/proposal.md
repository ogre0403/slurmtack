## Why

The current `openstack_to_slurm` workflow requires the caller to provide `node_name` up front and relies on the orchestrator's 2-second store polling loop to discover and advance work. That does not match the desired operating model, where the selected node is delivered through MQ and the orchestrator participates in MQ-driven coordination instead of repeatedly scanning the state store.

This change moves only the entry and dispatch mechanics for `openstack_to_slurm` to MQ while preserving the rest of the workflow after node binding. It gives operators a message-driven handoff point for node selection and removes the requirement that the control loop wake up just to find work in SQLite.

## What Changes

- **BREAKING** Change `openstack_to_slurm` request semantics so the API no longer depends on a caller-supplied `node_name` to start the execution.
- Add MQ-delivered node binding for `openstack_to_slurm`, so the selected node is correlated to the execution through RabbitMQ before node-bound orchestration begins.
- Change orchestrator control flow from periodic active-execution polling to MQ-driven registration and work intake, while preserving the downstream `openstack_to_slurm` steps after node identity is known.
- Keep the existing post-binding workflow intact: source quiesce verification, host reconfiguration, reboot, SSH reachability, Slurm attach, and verify remain unchanged.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `rest-api`: change `openstack_to_slurm` request behavior so execution creation no longer requires the node to be provided in the request body.
- `amqp-events`: add the MQ contract for delivering and correlating the selected `openstack_to_slurm` node to an execution, and define orchestrator-side MQ registration/consumption expectations.
- `orchestrator`: replace the tick-based store scan with MQ-driven work intake and update the control-path expectations for when actions are selected.
- `gpu-node-switch-allocation`: change how an `openstack_to_slurm` execution becomes node-bound so the node is attached through MQ before lease acquisition and node-bound actions begin.

## Impact

- Affected code: `cmd/main.go`, `internal/api`, `internal/service`, `internal/orchestrator`, `internal/mq`, `internal/store`, and related tests.
- Affected systems: RabbitMQ topology/consumers, execution state progression, and operator/client integration for `openstack_to_slurm` submission.
- API impact: `openstack_to_slurm` callers must stop treating `node_name` as the primary request input and instead publish or integrate with the MQ-based node handoff path.
- Operational impact: MQ becomes part of the required control path for starting `openstack_to_slurm` work; the orchestrator no longer depends on periodic store scans to discover work.