## ADDED Requirements

### Requirement: Asynchronous switch execution
The system SHALL accept a request to switch a GPU node between `slurm` and `openstack` and create a durable execution record before any host or control-plane mutation begins. Each execution MUST track the current switch state, desired owner, previous owner, request direction, and terminal outcome independently of the caller session.

#### Scenario: Request is accepted before mutation
- **WHEN** an operator submits a valid switch request
- **THEN** the system creates an execution in the `requested` state and returns an execution identifier without waiting for the full switch to complete

### Requirement: Versioned node-switch state machine
The system SHALL drive each execution through an explicit state machine with versioned transitions. The implementation MUST use the persisted state machine, rather than sequence timing alone, to determine whether the workflow may proceed, compensate, or terminate.

#### Scenario: Transition advances state version
- **WHEN** the daemon moves an execution from one non-terminal state to the next
- **THEN** it persists the new state together with an incremented `state_version`

#### Scenario: Invalid transition is rejected
- **WHEN** a step handler attempts to skip required intermediate states or resume from a mismatched persisted state
- **THEN** the system rejects the transition and records the execution as failed or blocked rather than applying the mutation

### Requirement: Exclusive per-node lease
The system SHALL require an exclusive per-node lease before any node-bound precheck, source detachment, host mutation, or target attachment begins. At most one active execution MAY hold the lease for a node at a time.

#### Scenario: Concurrent switch requests target the same node
- **WHEN** a second execution attempts to acquire the lease for a node with an active lease
- **THEN** the system refuses the lease and prevents the second execution from performing node-bound actions

### Requirement: Safe failure classification and compensation
The system SHALL classify failed steps as `transient`, `precheck_blocked`, `mutation_partial`, `verification_failed`, or `unknown_after_reboot`. If a failure occurs after ownership mutation starts, the system MUST either enter compensation with explicit rollback steps or mark the execution as requiring manual recovery.

#### Scenario: Failure occurs before ownership changes
- **WHEN** a precheck or source-quiescing action fails before the source owner is detached
- **THEN** the system marks the execution as `failed_non_destructive`

#### Scenario: Failure occurs after reboot with unknown host state
- **WHEN** the host does not return with a provable healthy state after reboot
- **THEN** the system marks the execution as `failed_manual_recovery` and preserves execution evidence

### Requirement: Live workflow control path emits action-selection traces
The system SHALL emit trace events from the live orchestrator control path whenever it evaluates an active execution and selects the next daemon action. The trace output MUST identify the execution, current state, selected action, and component responsible for that decision.

#### Scenario: Orchestrator selects the next action for a requested execution
- **WHEN** the orchestrator evaluates an active execution in a non-terminal state
- **THEN** it emits a trace entry that records the current execution state and the next selected action before that action runs

### Requirement: Execution terminal handling is traceable across success and failure paths
The system SHALL emit trace events when a workflow action fails, when the daemon classifies that failure, and when the execution reaches a terminal completed or failed state. Terminal trace entries MUST preserve the same execution correlation fields used earlier in the workflow.

#### Scenario: Failed action is classified and logged
- **WHEN** a workflow action or transition fails and the daemon maps the execution to a terminal failure class
- **THEN** the daemon emits trace entries for the failed action, the derived failure classification, and the terminal state that will be persisted

#### Scenario: Completion cleanup is logged
- **WHEN** the daemon releases the lease and finalizes an execution as completed
- **THEN** it emits trace entries that show the completion action, lease-release result, and final terminal state for that execution