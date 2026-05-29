## Context

Today the repo's active specs and code treat `openstack_to_slurm` as a two-step workflow: `POST /v1/switches` creates an execution in `awaiting_target_node`, then some separate RabbitMQ producer must publish `execution.node_selected` with the chosen host. The consumer binds the node and admits the execution into the existing orchestrator path.

The requested behavior is narrower than the earlier MQ-driven redesign. We do not want to remove the MQ-based correlation point inside the daemon; we want the REST API to become the producer for that correlation event when the caller already knows the target node. That keeps the orchestrator and MQ consumer behavior largely intact while removing the manual RMQ client step from operations.

## Goals / Non-Goals

**Goals:**
- Require `node_name` on `openstack_to_slurm` API requests.
- Persist the execution first, then publish the corresponding `execution.node_selected` event from the API/service path.
- Keep MQ as the orchestrator admission path for node binding so downstream behavior stays consistent with the current event-driven design.
- Preserve existing `slurm_to_openstack` request and placeholder-allocation behavior.

**Non-Goals:**
- Removing `awaiting_target_node` or bypassing MQ for `openstack_to_slurm` state progression.
- Redesigning the orchestrator state machine after the node is bound.
- Introducing a new queue, exchange, or alternate transport for node selection.
- Solving guaranteed publish durability beyond the current best-effort publisher behavior already used for `execution.requested`.

## Decisions

### 1. Keep `awaiting_target_node`, but populate it from API-submitted `node_name`

`openstack_to_slurm` executions will still be created in `awaiting_target_node` so the persisted state machine and consumer logic remain aligned with the existing event-driven contract. The difference is that the API request must include `node_name`, and the API/service layer will use that value to emit `execution.node_selected` immediately after persistence.

This preserves a visible wait boundary between persistence and MQ consumption, even if that boundary is usually brief.

Alternatives considered:
- Persist directly to `node_identified`. Rejected because it would bypass the MQ-driven orchestrator intake path and create a second admission mechanism for the same workflow.
- Remove `awaiting_target_node` only for API-created executions. Rejected because it would split one direction into two incompatible control paths.

### 2. Extend the publisher interface to support `execution.node_selected`

The service layer already publishes `execution.requested` after persistence. We will extend that publisher abstraction so the same path can also publish `execution.node_selected` with `execution_id` and `node_name` for `openstack_to_slurm` requests.

The service remains responsible for validation, persistence, and publish triggering. The MQ consumer remains responsible for binding the execution, advancing to `node_identified`, and admitting orchestrator work.

Alternatives considered:
- Publish `execution.node_selected` directly from the API handler. Rejected because publish behavior belongs with request persistence semantics in the service layer, not HTTP transport code.
- Reuse `execution.requested` with an embedded `node_name`. Rejected because the existing consumer contract treats requested and node-selection events as distinct concerns.

### 3. Fail request validation early when `openstack_to_slurm` omits `node_name`

Because the API is becoming the authoritative source for node selection in this workflow, `openstack_to_slurm` must reject requests that omit `node_name`. This removes the partial execution state that previously relied on a separate manual producer and matches the desired operator contract.

Alternatives considered:
- Accept missing `node_name` and keep the old manual MQ fallback. Rejected because it preserves the dual operational model the change is trying to remove.
- Accept `node_name` optionally and publish when present. Rejected because it leaves two supported ways to start the same workflow.

### 4. Preserve current publish-failure handling for consistency

The current `execution.requested` publish is best-effort: the execution is persisted first, and publish failures are logged without rolling back the request. This change will keep that model for `execution.node_selected` so the behavior stays consistent with existing submission semantics and avoids introducing distributed transaction logic into the API path.

Alternatives considered:
- Fail the whole request if `execution.node_selected` publish fails. Rejected for now because it would create inconsistent behavior between directions and leave behind persistence questions for already-written executions.
- Add an outbox table in this change. Rejected as too large for the requested workflow fix.

## Risks / Trade-offs

- [Publish failure leaves an `openstack_to_slurm` execution waiting in `awaiting_target_node`] -> Log the failure with execution and node context, and cover the behavior in tests so operators know the recovery path.
- [API and consumer now both depend on the same node name contract] -> Keep the event schema unchanged and validate `node_name` before persistence.
- [This reintroduces a breaking API change relative to the current spec] -> Update the REST, AMQP, and allocation specs together so the contract remains internally consistent.
- [Fast publish/consume may make `awaiting_target_node` short-lived] -> Keep the explicit state anyway because it remains the durable handoff boundary and recovery point.

## Migration Plan

1. Update the request contract and service validation so `openstack_to_slurm` requires `node_name`.
2. Extend the MQ publisher abstraction and implementation with `PublishNodeSelected`.
3. Publish `execution.node_selected` immediately after persisting an `openstack_to_slurm` execution.
4. Update API, service, MQ, and integration tests for the new submission contract and auto-publish behavior.
5. Deploy the API/service publisher change before relying on the new request contract operationally.
6. If rollback is required, revert to the previous version and resume manual `execution.node_selected` publishes for any executions left in `awaiting_target_node`.

## Open Questions

- Should the create response mention that node selection has already been queued, or is the existing `execution_id` plus status URL sufficient?
- Do we want follow-up work for reliable publish/outbox handling, now that both directions depend on API-side MQ publishing?
