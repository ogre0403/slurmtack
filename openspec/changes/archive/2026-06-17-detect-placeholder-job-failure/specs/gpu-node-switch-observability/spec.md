## ADDED Requirements

### Requirement: Allocation wait failures preserve operator-visible reasons

The system SHALL persist an operator-visible failure summary when a `slurm_to_openstack` execution fails during `awaiting_source_allocation`. When the placeholder job reaches a terminal state before an allocation event is received, the failed `wait_for_source_allocation` step and the execution-level failure summary MUST preserve a concise, deterministic reason that identifies the placeholder job when known and the observed Slurm state when available.

#### Scenario: Failed placeholder job records state-based summary

- **WHEN** a `slurm_to_openstack` execution fails from `awaiting_source_allocation` because placeholder job `12345` entered Slurm state `FAILED`
- **THEN** the failed `wait_for_source_allocation` step includes an operator-visible summary mentioning job `12345` and state `FAILED`
- **AND** the execution detail also exposes that summary as its final failure reason

#### Scenario: Placeholder job completion without allocation records a readable reason

- **WHEN** a `slurm_to_openstack` execution fails from `awaiting_source_allocation` because its placeholder job finished before any allocation event was received
- **THEN** the failed `wait_for_source_allocation` step includes a readable summary explaining that allocation never arrived
- **AND** the execution detail exposes the same operator-visible reason
