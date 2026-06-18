## Purpose

Define orchestrator workflow requirements for GPU node switch executions, including execution lifecycle, lease handling, failure classification, tracing, and guarded Slurm attachment.
## Requirements
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
The system SHALL classify failed steps as `transient`, `precheck_blocked`, `mutation_partial`, `verification_failed`, or `unknown_after_reboot`. If a failure occurs after ownership mutation starts, the system MUST either enter compensation with explicit rollback steps or mark the execution as requiring manual recovery. Failures raised during target attachment from `host_reachable` MUST resolve to a durable terminal failed state and MUST NOT leave the execution active in `host_reachable`.

#### Scenario: Failure occurs before ownership changes
- **WHEN** a precheck or source-quiescing action fails before the source owner is detached
- **THEN** the system marks the execution as `failed_non_destructive`

#### Scenario: Failure occurs after reboot with unknown host state
- **WHEN** the host does not return with a provable healthy state after reboot
- **THEN** the system marks the execution as `failed_manual_recovery` and preserves execution evidence

#### Scenario: OpenStack-to-Slurm attach failure after host reachability becomes terminal
- **WHEN** an `openstack_to_slurm` execution is in `host_reachable`
- **AND** the target-attach action fails before the workflow can persist `target_attaching`
- **THEN** the system marks the execution as `failed_needs_rollback`
- **AND** the execution does not remain active in `host_reachable`

#### Scenario: Slurm-to-OpenStack attach failure after host reachability becomes terminal
- **WHEN** a `slurm_to_openstack` execution is in `host_reachable`
- **AND** the target-attach action fails before the workflow can persist `target_attaching`
- **THEN** the system marks the execution as `failed_needs_rollback`
- **AND** the execution does not remain active in `host_reachable`

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

### Requirement: Guarded Slurm target attachment

For `openstack_to_slurm` executions, the system SHALL inspect the current Slurm node state before issuing a target-side `RESUME`. It MUST evaluate composite Slurm state strings token-by-token. If the node state includes `drain`, `drained`, or `down`, the system MUST issue `RESUME`. If the node is already schedulable in `idle`, `alloc`, or `mixed` and no drain/down token is present, the system MUST skip `RESUME` and continue the workflow. If the node is in any other state, the system MUST fail the attach step without issuing `RESUME`.

#### Scenario: Composite drain state resumes the node
- **WHEN** an `openstack_to_slurm` execution reaches target attachment and Slurm reports a state such as `drained+drain`, `idle+drain`, or `down`
- **THEN** the system issues `ResumeNode` for that node before continuing the workflow

#### Scenario: Already schedulable state skips resume
- **WHEN** an `openstack_to_slurm` execution reaches target attachment and Slurm reports `idle`, `alloc`, or `mixed` with no drain/down token
- **THEN** the system does not call `ResumeNode` and continues to verification using the current active node state

#### Scenario: Unsupported node state fails before mutation
- **WHEN** an `openstack_to_slurm` execution reaches target attachment and Slurm reports a node state that is neither resumable nor already schedulable
- **THEN** the system fails the attach step with an error describing the observed node state and does not call `ResumeNode`

### Requirement: Slurmd is quiesced before Slurm-to-OpenStack host mutation

For `slurm_to_openstack` executions, after the node has been drained through the Slurm API and before the daemon proceeds with host-side reconfiguration, the system SHALL use the configured SSH runner to execute `systemctl stop slurmd` and then `systemctl disable slurmd` on the target node. If either command fails, the workflow MUST fail and MUST NOT continue to host mutation or target attachment.

#### Scenario: Drained node stops and disables slurmd before reconfiguration
- **WHEN** a `slurm_to_openstack` execution has completed its Slurm drain workflow and is ready to leave source ownership
- **THEN** the daemon executes `systemctl stop slurmd` followed by `systemctl disable slurmd` through the SSH runner before transitioning deeper into host reconfiguration

#### Scenario: Slurmd shutdown failure blocks the handoff
- **WHEN** a `slurm_to_openstack` execution cannot stop or disable `slurmd` through the SSH runner
- **THEN** the daemon records the step as failed and does not continue to host mutation or OpenStack attachment

### Requirement: Slurmd is restored before OpenStack-to-Slurm attachment

For `openstack_to_slurm` executions, before the daemon evaluates Slurm attach readiness or issues `ResumeNode`, the system SHALL use the configured SSH runner to execute `systemctl enable slurmd` and then `systemctl start slurmd` on the target node. Because attachment runs shortly after a reboot, the daemon MUST tolerate boot-transient SSH failures on these commands: when a command fails because the target is still booting (for example `pam_nologin` reporting the system is booting up, or the SSH session closing during the login window), the daemon SHALL retry the command with bounded backoff before giving up. If a command still fails after the bounded retries, or fails for a non-transient reason, the workflow MUST fail and MUST NOT issue `ResumeNode` or declare the node ready for Slurm attachment. The harmless post-quantum key-exchange warning on SSH stderr MUST NOT by itself cause the command to be treated as failed.

#### Scenario: Slurmd enable and start happen before Slurm attach evaluation
- **WHEN** an `openstack_to_slurm` execution reaches target attachment after host reconfiguration
- **THEN** the daemon executes `systemctl enable slurmd` followed by `systemctl start slurmd` through the SSH runner before it evaluates node attach state or calls the Slurm resume API

#### Scenario: Boot-transient slurmd restore failure is retried then succeeds
- **WHEN** a `systemctl enable slurmd` or `systemctl start slurmd` command fails because the target is still booting (`pam_nologin`) and a subsequent retry within the bounded attempts succeeds
- **THEN** the daemon proceeds to evaluate node attach state and complete attachment without failing the execution

#### Scenario: Slurmd restore fails after bounded retries
- **WHEN** an `openstack_to_slurm` execution cannot enable or start `slurmd` through the SSH runner after the bounded retries, or fails for a non-transient reason
- **THEN** the daemon records the step as failed and does not issue `ResumeNode` or complete Slurm attachment

### Requirement: Guard duplicate OpenStack-to-Slurm request admission

Before creating an `openstack_to_slurm` execution, the system SHALL inspect the target node's current Slurm state and reject the request when that state already shows the node is back in active Slurm service. Active Slurm service MUST include schedulable states such as `idle`, `alloc`, or `mixed` when no `drain`, `drained`, or `down` token is present. When the request is rejected by this guard, the system MUST NOT persist an execution, publish MQ admission events, acquire a lease, or start any host mutation workflow.

#### Scenario: Active Slurm node is rejected before execution creation
- **WHEN** an operator submits `openstack_to_slurm` for node `gpu-01`
- **AND** Slurm reports `gpu-01` in `idle`
- **THEN** the system rejects the request as already under Slurm ownership
- **AND** no execution record is created

#### Scenario: Composite active state is rejected before execution creation
- **WHEN** an operator submits `openstack_to_slurm` for node `gpu-01`
- **AND** Slurm reports `gpu-01` in a schedulable state such as `mixed` with no drain/down token
- **THEN** the system rejects the request as already under Slurm ownership
- **AND** no MQ publish for that execution occurs

#### Scenario: Non-active Slurm node can still enter workflow
- **WHEN** an operator submits `openstack_to_slurm` for node `gpu-01`
- **AND** Slurm reports `gpu-01` in `drained`, `down`, or another resumable non-active state
- **THEN** the system accepts the request and continues the existing `openstack_to_slurm` workflow

#### Scenario: Slurm state cannot be determined at request time
- **WHEN** an operator submits `openstack_to_slurm` for node `gpu-01`
- **AND** the system cannot read the current Slurm node state
- **THEN** the request fails before execution creation
- **AND** the system does not start the switch workflow with incomplete ownership information

