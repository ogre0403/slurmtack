## Purpose

Define orchestrator control-path requirements for MQ-driven admission, state progression, failure handling, recovery, concurrency, and shutdown.

## Requirements

### Requirement: State-to-action mapping

The orchestrator SHALL map each `(current_state, direction, trigger)` combination to the correct next action. Triggers MAY come from MQ intake (`execution.requested`, `execution.node_selected`, `execution.allocation`, `execution.drained`), local wait completion, or startup recovery. Actions include: submit placeholder job, acquire lease, run prechecks, invoke step handlers (quiesce, detach, attach, verify), trigger reboot, SSH poll, and mark completed.

#### Scenario: Slurm-to-OpenStack from requested after MQ admission

- **WHEN** an execution is in `requested` with direction `slurm_to_openstack` and the orchestrator admits it from a matching `execution.requested` event
- **THEN** orchestrator submits a placeholder job via the Slurm client and transitions to `awaiting_source_allocation`

#### Scenario: OpenStack-to-Slurm waits for MQ node selection

- **WHEN** an execution is in `awaiting_target_node` with direction `openstack_to_slurm`
- **THEN** orchestrator does not run lease acquisition or any other node-bound action until a matching MQ node-selection event is correlated

#### Scenario: OpenStack-to-Slurm resumes after node selection

- **WHEN** an execution is in `node_identified` with direction `openstack_to_slurm`
- **THEN** orchestrator acquires the node lease and transitions to `locked` before continuing with existing node-bound actions

#### Scenario: Completed execution

- **WHEN** an execution is in `verifying` and the verification handler succeeds
- **THEN** orchestrator transitions to `completed`, releases the lease, and sets `overall_status` to succeeded

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

- **WHEN** context is cancelled while orchestrator is idle with no further work to admit
- **THEN** orchestrator exits the control path immediately

#### Scenario: Shutdown during step execution

- **WHEN** context is cancelled while a step handler is running
- **THEN** orchestrator waits for the current step to finish, then exits without starting new steps

### Requirement: Optimistic concurrency safety

The orchestrator SHALL handle `ErrVersionConflict` from the store gracefully. If admission, recovery, or event handling fails due to version conflict, the daemon MUST skip duplicate work and rely on the persisted winner state rather than failing the execution.

#### Scenario: Version conflict from duplicate MQ delivery

- **WHEN** two copies of the same MQ event race to advance one execution and one path gets `ErrVersionConflict`
- **THEN** the daemon logs the conflict, acks the stale work item, and leaves the execution active in the persisted winner state

#### Scenario: Version conflict from startup recovery versus live event

- **WHEN** startup recovery and a newly delivered MQ event race to advance the same execution
- **THEN** the losing path skips further mutation and trusts the current persisted state

### Requirement: OpenStack-to-Slurm source quiesce verification

The orchestrator SHALL actively re-evaluate `openstack_to_slurm` executions in `source_quiescing` while that execution remains active in the control path. It MUST verify that the host's OpenStack compute service is disabled and that the host has no resident instances or active migrations before transitioning the execution to `source_detached`.

#### Scenario: Source quiesce still in progress

- **WHEN** an `openstack_to_slurm` execution is in `source_quiescing` and the compute service is still enabled, or instances or active migrations are still present on the host
- **THEN** the orchestrator leaves the execution in `source_quiescing` and retries verification later within the same active control path

#### Scenario: Source quiesce verification succeeds

- **WHEN** an `openstack_to_slurm` execution is in `source_quiescing`, the compute service is disabled, and the host has no resident instances or active migrations
- **THEN** the orchestrator transitions the execution to `source_detached`

#### Scenario: Source quiesce verification query fails

- **WHEN** an `openstack_to_slurm` execution is in `source_quiescing` and the orchestrator cannot read the required OpenStack quiesce signals
- **THEN** the verification action fails and the orchestrator applies its normal step-failure handling for that execution

### Requirement: MQ-driven execution intake

The orchestrator SHALL start by activating its MQ subscriptions and SHALL use MQ events, rather than periodic active-execution polling, as the admission mechanism for new work.

#### Scenario: Startup activates MQ intake before processing work

- **WHEN** the daemon starts with MQ-driven orchestration enabled
- **THEN** the orchestrator activates the required MQ subscriptions before it begins admitting new executions into the workflow

#### Scenario: No repeating store scan is used for work discovery

- **WHEN** the daemon is idle with no new MQ events and no local wait loops in progress
- **THEN** the orchestrator does not wake up on a fixed interval to query all active executions from the store

### Requirement: One-time startup recovery for active executions

On startup, the orchestrator SHALL perform a single recovery scan of persisted active executions so that in-flight work can resume after restart without reintroducing continuous store polling.

#### Scenario: Recovery resumes a rebooting execution

- **WHEN** the daemon starts and finds an active execution in `rebooting`
- **THEN** it re-arms the SSH reachability wait for that execution

#### Scenario: Recovery leaves MQ-waiting execution parked

- **WHEN** the daemon starts and finds an active execution in `awaiting_target_node`, `awaiting_source_allocation`, or `source_quiescing` waiting only on MQ
- **THEN** it leaves that execution persisted in place and waits for the matching MQ event instead of mutating it during recovery

### Requirement: Cancellation claims override normal state progression

The orchestrator SHALL treat `cancelling` as a dedicated cleanup state rather than a normal workflow wait or mutation state. While an execution is in `cancelling`, the orchestrator MUST NOT run the ordinary action for the previously claimed wait state, and once an execution is `cancelled` it MUST NOT schedule any further workflow actions for that execution.

#### Scenario: Claimed allocation wait runs cancellation cleanup instead of waiting

- **WHEN** the orchestrator evaluates a `cancelling` execution whose recorded cancellation source state is `awaiting_source_allocation`
- **THEN** it selects the cancellation-cleanup action for that execution
- **AND** it does not resume the normal placeholder-allocation wait path

#### Scenario: Cancelled execution is skipped

- **WHEN** the orchestrator encounters an execution in `cancelled`
- **THEN** it does not schedule any additional workflow action for that execution

### Requirement: Startup recovery resumes in-progress cancellation

On startup, the orchestrator SHALL recover active executions already persisted in `cancelling`. It MUST use the recorded cancellation source state and direction to resume the correct cleanup path instead of restoring the original wait behavior.

#### Scenario: Recovery resumes source_quiescing cancellation cleanup

- **WHEN** the daemon starts and finds a `cancelling` execution whose recorded cancellation source state is `source_quiescing`
- **THEN** the orchestrator re-arms the source-quiesce cancellation cleanup for that execution
- **AND** it does not resume the original quiesce wait path
