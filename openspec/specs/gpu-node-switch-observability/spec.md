## ADDED Requirements

### Requirement: Structured execution and step records
The system SHALL persist one execution record per switch request and one step record per transition or action. Execution records MUST capture request metadata, ownership intent, state version, lock timing, and final error details. Step records MUST capture timing, retry count, exit code, error classification, and references to command output and snapshots.

#### Scenario: Step execution is auditable
- **WHEN** the daemon runs a precheck, drain, reboot, attach, or verification step
- **THEN** it persists a step record linked to the execution with the step status, timestamps, and output references

### Requirement: Deterministic per-execution evidence layout
The system SHALL write execution evidence to a deterministic log root at `/var/log/gpu-switch/<node_name>/<execution_id>/` and MUST include a manifest, event stream, step stdout or stderr logs, and host snapshots.

#### Scenario: Operator locates execution evidence
- **WHEN** an operator inspects a failed execution for a specific node and execution identifier
- **THEN** the expected manifest, event stream, step logs, and snapshot files exist under the deterministic log path

### Requirement: Reboot-boundary diagnostics
The system SHALL capture host boot identity before and after reboot, `journalctl -b` before reboot when available, `journalctl -b` and `journalctl -b -1` after reboot, and post-reboot diagnostics needed to explain service or GPU initialization failures.

#### Scenario: Host returns after reboot
- **WHEN** the daemon detects that a rebooted host is reachable again
- **THEN** it records the new boot ID and collects current-boot and previous-boot diagnostics before continuing verification

### Requirement: Request, event, and control-plane traceability
The system SHALL retain the user request payload, derived execution plan, message-bus publish or consume metadata, and Slurm or OpenStack request and response evidence for the lifetime of the execution record.

#### Scenario: Operator reconstructs a failed transition
- **WHEN** an execution fails during drain, detach, attach, or verification
- **THEN** the stored execution evidence shows the triggering request, the emitted and consumed events, and the related control-plane interactions for that execution

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