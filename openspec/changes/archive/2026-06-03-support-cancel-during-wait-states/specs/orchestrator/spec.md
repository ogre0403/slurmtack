## ADDED Requirements

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
