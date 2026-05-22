## Why

The API can create executions and the engine can execute steps, but nothing connects them. Currently tests drive the state machine manually. In production the daemon needs: (1) an orchestrator goroutine that automatically advances executions through the state machine by invoking the correct step handlers, and (2) an AMQP consumer that receives async events from the placeholder job (allocation_event, node_drained) and feeds them into the orchestrator. Without these, the daemon is a passive record keeper that cannot actually perform switches.

## What Changes

- Add an orchestrator goroutine that picks up active executions and drives them through the state machine step by step
- Add an AMQP consumer that subscribes to RabbitMQ for `allocation_event` and `node_drained` messages from the placeholder job
- Add an AMQP publisher interface (used by the placeholder-agent in Change 5, wired here for topology setup)
- Declare RabbitMQ exchange and queue topology on daemon startup
- Add SSH poll loop for host-reachable detection after reboot (replaces MQ-based host_reachable event)
- Wire orchestrator into `cmd/main.go` alongside the API server

## Capabilities

### New Capabilities

- `orchestrator`: In-process goroutine that evaluates active executions, determines the next action based on current state and direction, invokes step handlers, transitions state, and handles failures/compensation
- `amqp-events`: RabbitMQ consumer for placeholder job events (allocation_event, node_drained), exchange/queue topology declaration, and message schema definitions
- `ssh-reachability`: SSH poll loop that detects when a rebooted host becomes reachable again, replacing the MQ-based host_reachable event from the design doc

### Modified Capabilities

(none)

## Impact

- **New packages**: `internal/orchestrator/`, `internal/mq/`
- **Modified packages**: `cmd/` (wire orchestrator and MQ consumer into main)
- **New dependencies**: `github.com/rabbitmq/amqp091-go` (AMQP 0.9.1 client)
- **External systems**: RabbitMQ (already in docker-compose from Change 1)
- **Configuration**: Uses `AMQP_URL` env var (already stubbed), adds `SSH_POLL_INTERVAL`, `SSH_POLL_TIMEOUT`
