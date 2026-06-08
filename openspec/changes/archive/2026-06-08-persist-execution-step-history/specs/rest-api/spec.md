## ADDED Requirements

### Requirement: Query execution step timeline
The system SHALL return a durable execution step timeline via `GET /v1/switches/:id/steps` for an existing execution. The response MUST include every persisted step record for that execution in ascending `sequence` order, including currently running action or wait steps. Each step item MUST include `sequence`, `step_name`, `status`, `started_at`, `ended_at` when present, `host` when present, `retry_count`, `exit_code` when present, `error_class` when present, `command_id` when present, and any recorded stdout, stderr, or snapshot paths.

#### Scenario: Active execution exposes prior and current persisted steps
- **WHEN** client sends `GET /v1/switches/:id/steps` for an execution that has already persisted completed steps and is currently waiting in `awaiting_source_allocation` or `rebooting`
- **THEN** the system returns HTTP 200 with the prior completed steps followed by the current running wait step
- **AND** the running step omits `ended_at` until that wait finishes

#### Scenario: Completed execution exposes full ordered history
- **WHEN** client sends `GET /v1/switches/:id/steps` for a completed, failed, or cancelled execution with persisted runtime history
- **THEN** the system returns HTTP 200 with all of that execution's step records ordered by ascending `sequence`
- **AND** the response preserves failure and evidence metadata recorded on individual steps

#### Scenario: Query step timeline for unknown execution
- **WHEN** client sends `GET /v1/switches/:id/steps` for a non-existent execution ID
- **THEN** the system returns HTTP 404 with an error indicating that the execution was not found
