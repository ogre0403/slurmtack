## Context

After Changes 1-3, the daemon has: a REST API that creates executions, a SQLite store, and real Slurm/OpenStack clients. But nothing drives executions forward — the state machine only advances when code explicitly calls `Runner.Transition` and `Runner.RunStep`. In tests this is manual; in production an orchestrator must do it automatically.

The design doc specifies two async events from the placeholder job (allocation_event, node_drained) delivered via RabbitMQ, and a host-reachable detection after reboot. We decided to use SSH polling for reachability instead of a MQ event from the node.

## Goals / Non-Goals

**Goals:**

- Orchestrator goroutine that continuously processes active executions
- AMQP consumer for placeholder job events with idempotent handling
- SSH poll loop for post-reboot reachability
- RabbitMQ topology declaration (exchange, queues, bindings) on startup
- Clean shutdown integration with existing graceful shutdown

**Non-Goals:**

- Horizontal scaling / distributed orchestrator (single daemon instance)
- Dead letter queues or complex retry topology (simple nack + requeue)
- Metrics/tracing integration (future change)
- Placeholder job implementation (Change 5)
- Webhook or SSE push notifications to API consumers

## Decisions

### Orchestrator Pattern: Poll-based Loop

**Choice**: A single goroutine running a tick-based loop that queries active executions and processes them one at a time.

**Alternatives considered**:
- Event-driven (channel per execution): More complex, risk of goroutine leaks, harder to reason about shutdown
- Worker pool: Overkill for single-daemon with at most a handful of concurrent executions

**Rationale**: GPU node switches happen infrequently (a few per day). A single goroutine with a 1-2 second tick interval is simple, predictable, and easy to debug. Each tick:

```
loop (every 2s):
  executions = store.ListActiveExecutions()
  for each exec:
    nextAction = determineNextAction(exec)
    execute(nextAction)
    transition(exec)
```

If a step takes time (e.g., SSH poll), it blocks the loop. This is acceptable because switches are serialized per-node by the lease anyway, and only a few nodes switch concurrently.

### State-to-Action Mapping

The orchestrator maps `(current_state, direction)` to the next action:

```
requested + slurm_to_openstack    → submit placeholder job, transition to awaiting_source_allocation
requested + openstack_to_slurm    → acquire lease, run prechecks
awaiting_source_allocation        → wait for MQ allocation_event (no-op in tick)
node_identified                   → acquire lease, run prechecks
locked                            → run prechecks
precheck_passed                   → run source quiesce handler
source_quiescing                  → wait for MQ node_drained (s2o) or verify quiesce (o2s)
source_detached                   → run host reconfigure step
host_reconfiguring                → trigger reboot (if needed) or skip to target_attaching
rebooting                         → SSH poll loop until reachable
host_reachable                    → run target attach handler
target_attaching                  → run verification handler
verifying                         → mark completed, release lease
```

States that require MQ events (`awaiting_source_allocation`, waiting for `node_drained`) are skipped in the tick loop — the MQ consumer handles advancing those.

### AMQP: amqp091-go Library

**Choice**: `github.com/rabbitmq/amqp091-go` (official RabbitMQ Go client)

**Alternatives considered**:
- streadway/amqp: Deprecated, forked into amqp091-go
- NATS/Redis Streams: Would work but RabbitMQ is already in the stack

**Rationale**: amqp091-go is the maintained fork of the original streadway/amqp. Direct AMQP 0.9.1 support, widely used, lightweight.

### MQ Topology

```
Exchange: gpu-switch.events (topic, durable)

Queues:
  gpu-switch.allocation  ← binding key: execution.allocation
  gpu-switch.drained     ← binding key: execution.drained

Message format (JSON):
  allocation_event:
    { "execution_id": "...", "job_id": "...", "node_name": "..." }

  node_drained:
    { "execution_id": "...", "node_name": "..." }
```

Consumer uses manual ack. Messages are acked after the state transition succeeds. On failure: nack with requeue (up to a limit, then dead-letter).

### MQ Event Processing

When an event arrives:

1. Validate `execution_id` exists and is in the expected state
2. If state doesn't match (stale/duplicate), ack and discard
3. If valid, apply the event (bind node, advance state)
4. Ack the message

This satisfies the design doc's idempotency rules: events with wrong `execution_id` or `state_version` are silently discarded.

### SSH Reachability: Poll with Backoff

**Choice**: After transitioning to `rebooting`, the orchestrator enters an SSH poll loop for that execution:

```
interval: 10s (configurable via SSH_POLL_INTERVAL)
timeout:  10min (configurable via SSH_POLL_TIMEOUT)
method:   attempt SSH connection + run `hostname` command
success:  transition to host_reachable
timeout:  fail execution as unknown_after_reboot
```

**Alternatives considered**:
- MQ event from node (requires agent installed on every node, adds complexity)
- ICMP ping (doesn't prove SSH/services are up)

**Rationale**: SSH poll is self-contained — no agent needed on the node. The orchestrator already has SSH access via `remote.Runner`. 10-second intervals are inexpensive and detect reboot completion within ~10s of the host coming back.

### Concurrency Model

```
main goroutine
├── HTTP server (Gin)
├── Orchestrator goroutine (tick loop)
├── MQ consumer goroutine (blocks on channel.Consume)
└── signal handler (triggers shutdown)

Shutdown order:
1. Stop accepting HTTP requests
2. Stop MQ consumer (close channel)
3. Stop orchestrator (signal via context cancel)
4. Wait for in-flight step to complete (or timeout)
5. Close DB
```

## Risks / Trade-offs

- **Single-goroutine orchestrator blocks on long steps** → If SSH poll takes 10 minutes, other executions wait. Mitigation: acceptable for low concurrency; can add per-execution goroutines later if needed.
- **MQ connection loss** → If RabbitMQ restarts, consumer dies. Mitigation: reconnect loop with exponential backoff. Executions in `awaiting_source_allocation` will resume when consumer reconnects.
- **Ordering between MQ events and orchestrator tick** → Possible race where orchestrator and MQ consumer both try to advance the same execution. Mitigation: optimistic concurrency (state_version) in the store guarantees only one wins; the other gets ErrVersionConflict and retries on next tick.
- **SSH poll hammering a down host** → 10s interval for up to 10min = 60 attempts. Mitigation: this is a single TCP SYN per attempt, negligible load.
