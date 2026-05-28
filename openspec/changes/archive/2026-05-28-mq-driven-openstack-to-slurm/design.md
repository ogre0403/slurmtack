## Context

Today `openstack_to_slurm` starts from an API request that already contains `node_name`, and the orchestrator discovers work by polling the store every 2 seconds. RabbitMQ is only used for the `slurm_to_openstack` placeholder lifecycle (`execution.allocation` and `execution.drained`).

The requested change moves the first control point of `openstack_to_slurm` into MQ: the selected node is no longer part of the API request contract, and the orchestrator must activate against MQ at startup and consume node-selection signals instead of repeatedly scanning SQLite for runnable executions. The downstream workflow after the node is known must stay intact.

The design must preserve the persisted state machine, optimistic concurrency, and current single-daemon operating model. SQLite remains the system of record; MQ becomes the intake and correlation path.

## Goals / Non-Goals

**Goals:**
- Allow `openstack_to_slurm` requests to be created without `node_name`.
- Introduce an explicit waiting state for `openstack_to_slurm` until MQ supplies the node.
- Replace periodic store polling with MQ-driven execution intake plus targeted recovery on startup.
- Reuse the existing node-bound workflow after node binding: lease, precheck, source quiesce, host reconfigure, reboot, Slurm attach, verify.
- Keep the existing Slurm placeholder MQ events for `slurm_to_openstack`.

**Non-Goals:**
- Changing the host mutation logic after node binding.
- Redesigning the Slurm placeholder agent or its `execution.allocation` / `execution.drained` contract.
- Adding multi-daemon coordination or distributed leader election.
- Removing SQLite persistence or replacing the state machine with an in-memory-only workflow.

## Decisions

### 1. Add an explicit `awaiting_target_node` state for `openstack_to_slurm`

`openstack_to_slurm` requests will no longer be created with a bound node. Instead, the API will persist the execution in a new pre-binding state named `awaiting_target_node` with an empty `node_name`.

When MQ delivers a matching node-selection message, the daemon will record the node and transition the execution to `node_identified`. From there, the existing node-bound path resumes with lease acquisition.

Alternatives considered:
- Keep the execution in `requested` until MQ arrives. Rejected because the status view cannot distinguish "accepted but waiting for MQ node binding" from "accepted and runnable immediately".
- Bind the node and acquire the lease in one non-persisted step. Rejected because node correlation and lease acquisition are separate failure domains and should be observable independently.

### 2. Add MQ contracts for request intake and node selection

The daemon will use `gpu-switch.events` as the single exchange and extend topology with two additional routing keys:

- `execution.requested`
- `execution.node_selected`

The associated durable queues are:

- `gpu-switch.requested`
- `gpu-switch.node-selected`

Message schemas:

```json
{
  "execution_id": "<id>",
  "direction": "slurm_to_openstack|openstack_to_slurm"
}
```

```json
{
  "execution_id": "<id>",
  "node_name": "gpu-01"
}
```

`execution.requested` becomes the admission signal for new executions. `execution.node_selected` is only used for `openstack_to_slurm` node binding.

Alternatives considered:
- Reuse `execution.allocation` for `openstack_to_slurm`. Rejected because placeholder allocation and operator-selected target binding come from different producers and have different semantics.
- Skip `execution.requested` and trigger only from `execution.node_selected`. Rejected because removing the polling loop requires an explicit admission signal for workflows that do not start with node selection.

### 3. Replace the global tick loop with MQ-driven execution workers

The orchestrator will no longer run a repeating `ListActiveExecutions` loop. On startup it will activate MQ consumers, then dispatch per-execution workers when it receives a valid `execution.requested`, `execution.node_selected`, `execution.allocation`, or `execution.drained` event.

Each worker will:

1. Load the execution from the store.
2. Validate that the triggering event is valid for the current state.
3. Advance the execution until it reaches the next wait boundary or a terminal state.

Wait boundaries remain direction-specific:

- `awaiting_source_allocation` waits for MQ allocation.
- `awaiting_target_node` waits for MQ node selection.
- `source_quiescing` for `slurm_to_openstack` waits for MQ drained.
- `source_quiescing` for `openstack_to_slurm` continues to poll OpenStack control-plane state locally.
- `rebooting` continues to poll SSH reachability locally.

This preserves the existing downstream step logic while removing SQLite as the repeated work-discovery mechanism.

Alternatives considered:
- Keep the 2-second tick loop and add one more MQ event for node binding. Rejected because it does not satisfy the requested shift away from periodic store polling.
- Spawn a permanently running goroutine per active execution. Rejected because restart recovery and shutdown semantics become harder to reason about.

### 4. Perform one startup recovery scan instead of continuous store polling

Removing periodic polling creates a restart problem: active executions already persisted in SQLite must resume after daemon restart.

To solve this, the daemon will run a one-time recovery pass during startup after MQ consumers are registered. The recovery pass will inspect active executions and re-arm only the states that need local continuation:

- `node_identified`, `locked`, `precheck_passed`, `source_detached`, `host_reconfiguring`, `host_reachable`, `target_attaching`, `verifying`
- `source_quiescing` for `openstack_to_slurm`
- `rebooting`

Executions waiting only for MQ (`awaiting_target_node`, `awaiting_source_allocation`, `source_quiescing` for `slurm_to_openstack`) will not be mutated during recovery; they simply remain persisted until the corresponding MQ event arrives.

Alternatives considered:
- No recovery scan. Rejected because a daemon restart would strand active executions until external manual intervention.
- Keep the tick loop only for recovery. Rejected because a one-shot recovery pass is simpler and avoids reintroducing continuous polling semantics.

### 5. Keep API and MQ as separate sources of truth for different concerns

The API remains responsible for authentication, request validation, and execution creation. MQ becomes the transport for admission and node correlation, but not the persistence layer. Every worker must re-read the execution from SQLite before mutating it, and all duplicate or stale events must be handled idempotently through the existing optimistic concurrency model.

This keeps durable execution history in one place and prevents RabbitMQ from becoming the canonical workflow state store.

## Risks / Trade-offs

- [MQ becomes a harder dependency for workflow admission] -> Validate MQ activation before accepting MQ-driven work and fail request submission clearly when the intake path is unavailable.
- [Duplicate or reordered MQ messages could start duplicate work] -> Re-read persisted execution state before every mutation, use optimistic concurrency, and treat stale messages as ack-and-discard.
- [Startup recovery may race with freshly arriving MQ events] -> Use the existing state version checks and per-execution single-flight worker guard so only one path wins.
- [Introducing `awaiting_target_node` changes visible API/status behavior] -> Document the new state explicitly and keep all post-binding states unchanged.
- [The new requested-event path broadens MQ scope beyond current placeholder events] -> Reuse the existing exchange and manual-ack discipline to avoid introducing a second transport model.

## Migration Plan

1. Add the new persisted state (`awaiting_target_node`) and extend transition rules to allow `openstack_to_slurm` to wait for MQ node binding before lease acquisition.
2. Extend MQ topology with `execution.requested` and `execution.node_selected`, while keeping `execution.allocation` and `execution.drained` unchanged.
3. Update API/service submission so persisted executions emit `execution.requested`, and update `openstack_to_slurm` validation so node binding happens only through MQ.
4. Replace the repeating orchestrator tick loop with MQ-driven worker dispatch plus one-time startup recovery.
5. Roll out producer and consumer changes together so new `openstack_to_slurm` executions are not persisted into a state that no running daemon can admit.
6. If rollback is required, redeploy the previous daemon and republish any `execution.requested` messages for executions created during the mixed rollout window.

## Open Questions

- Which upstream component is authoritative for publishing `execution.node_selected` for `openstack_to_slurm`?
- Should `openstack_to_slurm` reject a supplied `node_name` with HTTP 400, or accept it only for backward-compatibility while ignoring it? This design assumes rejection to avoid dual sources of truth.
- Should the daemon reject all new switch requests when MQ intake is unavailable, or only reject the directions that depend on MQ-driven admission?