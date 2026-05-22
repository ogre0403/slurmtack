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