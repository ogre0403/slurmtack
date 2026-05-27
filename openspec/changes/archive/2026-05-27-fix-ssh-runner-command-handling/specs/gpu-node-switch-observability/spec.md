## ADDED Requirements

### Requirement: SSH dispatch logs show the rendered remote command

The system SHALL emit a structured trace log immediately before the SSH runner starts a remote command. The trace entry MUST record the target host, the rendered remote command string, and any available correlation metadata such as `execution_id` and `step_name`. The trace entry MUST NOT include private key paths or raw local SSH option values.

#### Scenario: Reboot dispatch is logged before execution

- **WHEN** the daemon dispatches the reboot action through the SSH runner
- **THEN** it emits a trace entry before process start that includes the reboot target host, the rendered remote command `reboot`, and the execution correlation fields for that reboot step

#### Scenario: Reachability probe dispatch is logged before execution

- **WHEN** the orchestrator dispatches a post-reboot reachability probe through the SSH runner
- **THEN** it emits a trace entry before process start that includes the probe target host, the rendered remote command `hostname`, and a stable step name for the probe attempt

#### Scenario: Local SSH transport details are not leaked

- **WHEN** the SSH runner logs a dispatch event
- **THEN** the log contains the target host and rendered remote command but omits private key paths and raw local SSH option values