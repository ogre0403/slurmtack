## 1. State Model and Request Contract

- [x] 1.1 Add the `awaiting_target_node` state and transition rules needed for `openstack_to_slurm` to wait for MQ node binding before lease acquisition.
- [x] 1.2 Update `openstack_to_slurm` request validation and execution creation so the API persists requests without `node_name`, rejects request-body `node_name`, and reports the new waiting state in status responses.
- [x] 1.3 Emit an `execution.requested` signal after execution persistence so new work can be admitted without periodic store polling.

## 2. MQ Topology and Event Handling

- [x] 2.1 Extend MQ topology, message types, and startup wiring with `gpu-switch.requested` / `execution.requested` and `gpu-switch.node-selected` / `execution.node_selected`.
- [x] 2.2 Implement idempotent handling for `execution.requested` and `execution.node_selected`, including binding `node_name` and transitioning `openstack_to_slurm` executions from `awaiting_target_node` to `node_identified`.
- [x] 2.3 Add MQ-focused tests for valid, duplicate, stale, and unknown requested/node-selection events.

## 3. Orchestrator Control Path

- [x] 3.1 Replace the repeating `ListActiveExecutions` tick loop with MQ-driven execution admission and per-execution dispatch.
- [x] 3.2 Update orchestrator state-to-action mapping so `openstack_to_slurm` waits in `awaiting_target_node`, resumes from `node_identified`, and continues the existing downstream workflow unchanged.
- [x] 3.3 Implement one-time startup recovery for persisted active executions, including restart handling for `rebooting` and `openstack_to_slurm` `source_quiescing` states.

## 4. Verification and Documentation

- [x] 4.1 Update API, orchestrator, and integration tests to cover the MQ-driven `openstack_to_slurm` admission path and the removal of periodic store-based work discovery.
- [x] 4.2 Update operator-facing documentation to explain the new MQ-driven `openstack_to_slurm` startup flow, required events, and the meaning of `awaiting_target_node`.
- [x] 4.3 Run the relevant OpenSpec validation and targeted Go tests for the changed API, MQ, and orchestrator slices.