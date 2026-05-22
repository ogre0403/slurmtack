## 1. Dependencies and Store Extension

- [x] 1.1 Add `github.com/rabbitmq/amqp091-go` to go.mod
- [x] 1.2 Add `ListActiveExecutions(ctx) ([]*Execution, error)` method to Store interface and implement in both memory and SQLite stores (query where overall_status = "active")
- [x] 1.3 Add `SSH_POLL_INTERVAL` and `SSH_POLL_TIMEOUT` to config struct with defaults

## 2. AMQP Module

- [x] 2.1 Create `internal/mq/connection.go` with connection management (connect, reconnect with backoff, close)
- [x] 2.2 Create `internal/mq/topology.go` with exchange and queue declarations (gpu-switch.events exchange, gpu-switch.allocation queue, gpu-switch.drained queue, bindings)
- [x] 2.3 Define message types in `internal/mq/messages.go` (AllocationEvent, NodeDrainedEvent structs with JSON tags)
- [x] 2.4 Create `internal/mq/consumer.go` with consumer goroutine: subscribe, dispatch by routing key, manual ack/nack
- [x] 2.5 Implement allocation_event handler: validate, bind node to execution, transition awaiting_source_allocation → node_identified
- [x] 2.6 Implement node_drained handler: validate, transition source_quiescing → source_detached
- [x] 2.7 Add message validation (malformed JSON → ack+discard, missing fields → ack+discard, unknown execution → ack+discard)
- [x] 2.8 Add version conflict handling (ErrVersionConflict → ack, not requeue)

## 3. SSH Reachability

- [x] 3.1 Create `internal/orchestrator/reachability.go` with SSH poll function: loop at interval, attempt SSH `hostname`, return on success or timeout
- [x] 3.2 Use existing `remote.Runner` interface for SSH attempts with 5s per-attempt timeout
- [x] 3.3 Handle context cancellation (shutdown during poll)

## 4. Orchestrator

- [x] 4.1 Create `internal/orchestrator/orchestrator.go` with constructor accepting Store, Runner, Slurm Client, OpenStack Client, DirectionHandlers, and config
- [x] 4.2 Implement tick loop: query active executions, process each, sleep 2s, respect context cancellation
- [x] 4.3 Implement state-to-action mapping for slurm_to_openstack direction (requested → submit placeholder, node_identified → acquire lease + precheck, precheck_passed → quiesce, source_detached → reconfigure, etc.)
- [x] 4.4 Implement state-to-action mapping for openstack_to_slurm direction (requested → acquire lease, locked → precheck, precheck_passed → quiesce, source_detached → reconfigure, etc.)
- [x] 4.5 Implement failure handling: catch step errors, classify failure, call FailExecution
- [x] 4.6 Implement ErrVersionConflict handling (skip + log, continue to next execution)
- [x] 4.7 Implement SSH poll integration for `rebooting` state (call reachability poll, transition or fail)

## 5. Integration and Wiring

- [x] 5.1 Wire orchestrator into `cmd/main.go`: start goroutine after store and clients are initialized
- [x] 5.2 Wire MQ consumer into `cmd/main.go`: connect, declare topology, start consumer goroutine
- [x] 5.3 Update graceful shutdown: cancel orchestrator context, close MQ consumer, wait for in-flight step
- [x] 5.4 Write integration test: create execution via API, send MQ event, verify state advances
- [x] 5.5 Write unit test: orchestrator tick with fake store and handlers, verify correct action dispatched per state
