## MODIFIED Requirements

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
