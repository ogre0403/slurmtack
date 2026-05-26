## ADDED Requirements

### Requirement: Execution-scoped daemon trace logs
The system SHALL emit structured daemon trace logs for each switch workflow step and asynchronous wait boundary. Each trace entry MUST identify the active execution with `execution_id`, `direction`, and `current_state`; it MUST include the workflow `action` or `step_name`, and it MUST include `node_name` once the execution is bound to a node.

#### Scenario: Workflow action entry and outcome are traceable
- **WHEN** the daemon starts or finishes a workflow action such as placeholder submission, lease acquisition, precheck, source quiesce, host reconfiguration, reboot, target attach, verification, or completion
- **THEN** it emits execution-scoped trace entries for the action start and the corresponding success or failure outcome

#### Scenario: Waiting periods are no longer silent
- **WHEN** an execution is waiting for placeholder allocation, drained confirmation, or host reachability after reboot
- **THEN** the daemon emits trace entries showing the wait condition, subsequent progress or retry attempts, and the final satisfied or timeout outcome for that wait

### Requirement: State transition logs identify attempted and persisted state changes
The system SHALL emit trace logs for state transition attempts and outcomes. Transition logs MUST identify the prior state, the requested next state, and whether the transition succeeded, was rejected as invalid, or was superseded by failure handling.

#### Scenario: Transition success is logged with state context
- **WHEN** the engine advances an execution from one persisted state to the next valid state
- **THEN** the daemon emits a trace entry that records the `from_state`, `to_state`, and execution correlation fields for that transition

#### Scenario: Transition rejection is logged for debugging
- **WHEN** the engine refuses an invalid state transition or cannot persist the state change
- **THEN** the daemon emits a trace entry that identifies the attempted states and the failure reason for that execution