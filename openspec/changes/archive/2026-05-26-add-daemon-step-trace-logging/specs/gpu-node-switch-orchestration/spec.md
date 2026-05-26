## ADDED Requirements

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