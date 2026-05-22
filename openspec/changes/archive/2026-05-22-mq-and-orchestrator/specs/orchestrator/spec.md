## ADDED Requirements

### Requirement: Tick-based execution loop

The orchestrator SHALL run a goroutine that ticks every 2 seconds, queries the store for active executions, and processes each one by determining and executing the next action based on current state and direction.

#### Scenario: Advance execution on tick

- **WHEN** an active execution is in state `precheck_passed` with direction `slurm_to_openstack`
- **THEN** the orchestrator invokes the source quiesce handler and transitions to `source_quiescing` on success

#### Scenario: Skip waiting executions

- **WHEN** an active execution is in state `awaiting_source_allocation`
- **THEN** the orchestrator skips it (MQ consumer will advance it when allocation_event arrives)

#### Scenario: No active executions

- **WHEN** the store has no active executions
- **THEN** the tick completes immediately and waits for the next interval

### Requirement: State-to-action mapping

The orchestrator SHALL map each `(current_state, direction)` pair to the correct next action. Actions include: submit placeholder job, acquire lease, run prechecks, invoke step handlers (quiesce, detach, attach, verify), trigger reboot, SSH poll, and mark completed.

#### Scenario: Slurm-to-OpenStack from requested

- **WHEN** execution is in `requested` with direction `slurm_to_openstack`
- **THEN** orchestrator submits a placeholder job via the Slurm client and transitions to `awaiting_source_allocation`

#### Scenario: OpenStack-to-Slurm from requested

- **WHEN** execution is in `requested` with direction `openstack_to_slurm`
- **THEN** orchestrator acquires the node lease and transitions to `locked`

#### Scenario: Completed execution

- **WHEN** execution is in `verifying` and verification handler succeeds
- **THEN** orchestrator transitions to `completed`, releases the lease, and sets overall_status to succeeded

### Requirement: Failure handling in orchestrator

The orchestrator SHALL catch errors from step handlers and invoke `Runner.FailExecution` with the appropriate failure class. It MUST NOT crash or stop processing other executions when one fails.

#### Scenario: Step fails pre-mutation

- **WHEN** a step handler returns an error while execution is in `precheck_passed`
- **THEN** orchestrator classifies as pre-mutation failure and transitions to `failed_non_destructive`

#### Scenario: Step fails post-mutation

- **WHEN** a step handler returns an error while execution is in `host_reconfiguring`
- **THEN** orchestrator classifies as mutation failure and transitions to `failed_needs_rollback`

#### Scenario: One execution fails, others continue

- **WHEN** execution A fails during its step
- **THEN** orchestrator logs the failure for A and continues processing execution B on the next iteration

### Requirement: Graceful shutdown

The orchestrator SHALL stop processing when its context is cancelled. If a step is in-flight, it MUST wait for it to complete (or timeout) before returning.

#### Scenario: Shutdown during idle

- **WHEN** context is cancelled while orchestrator is sleeping between ticks
- **THEN** orchestrator exits the loop immediately

#### Scenario: Shutdown during step execution

- **WHEN** context is cancelled while a step handler is running
- **THEN** orchestrator waits for the current step to finish, then exits without starting new steps

### Requirement: Optimistic concurrency safety

The orchestrator SHALL handle `ErrVersionConflict` from the store gracefully. If a transition fails due to version conflict (e.g., MQ consumer already advanced it), the orchestrator MUST skip that execution and retry on the next tick.

#### Scenario: Version conflict from concurrent MQ advance

- **WHEN** orchestrator attempts a transition but gets ErrVersionConflict
- **THEN** orchestrator logs the conflict and moves to the next execution without failing
