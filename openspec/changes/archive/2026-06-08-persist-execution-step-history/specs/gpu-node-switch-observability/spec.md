## MODIFIED Requirements

### Requirement: Structured execution and step records
The system SHALL persist one execution record per switch request and one durable step record for each live workflow action or asynchronous wait boundary that an execution enters. The runtime orchestrator, MQ intake path, and cancellation or failure handling MUST all write to that same persisted step timeline instead of relying only on structured logs or code paths that are not used in deployment. Step records MUST capture sequence ordering, timing, status, retry count, host when known, exit code when applicable, failure classification when applicable, and references to command output or snapshots when that evidence exists.

#### Scenario: Live workflow action is auditable in deployed execution history
- **WHEN** the daemon runs a real runtime action such as placeholder submission, lease acquisition, precheck, source quiesce, host reconfiguration, reboot, target attach, verification, completion, or cancel cleanup
- **THEN** it persists a step record linked to the execution
- **AND** it updates that record with the action outcome instead of leaving execution history empty

#### Scenario: Asynchronous wait boundary remains visible in the timeline
- **WHEN** an execution enters a wait boundary such as `awaiting_target_node`, `awaiting_source_allocation`, `source_quiescing`, or `rebooting`
- **THEN** the daemon persists a running wait step for that execution immediately
- **AND** it closes that same step as succeeded, failed, or skipped when the execution leaves the wait boundary through MQ intake, polling, cancellation, or terminal failure handling

#### Scenario: Terminal outcome closes any in-flight step
- **WHEN** an execution is cancelled or fails while its latest step is still running
- **THEN** the daemon updates the in-flight step with an ended timestamp and terminal status
- **AND** the step record captures the applicable failure classification when the execution ended due to an error
