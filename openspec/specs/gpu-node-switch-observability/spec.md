## ADDED Requirements

### Requirement: Blocked precheck steps preserve operator-visible reasons

The system SHALL persist an operator-visible rejection summary on failed precheck steps. When the daemon has structured blocker data, the stored step record MUST preserve a deterministic summary that identifies those blockers in stable wording and order. When precheck fails from a direct dependency or control-plane error instead of a structured blocker set, the stored failed precheck step MUST still preserve a concise operator-visible reason derived from that failure. This applies to `openstack_to_slurm` and `slurm_to_openstack` precheck failures that terminate the execution before mutation.

#### Scenario: Precheck records resident-instance blocker summary

- **WHEN** `openstack_to_slurm` precheck fails because the selected host still has resident instances
- **THEN** the persisted failed precheck step includes a stable rejection summary describing that resident-instance blocker

#### Scenario: Precheck records multiple blockers in stable order

- **WHEN** `openstack_to_slurm` precheck fails because the selected host has both resident instances and active migrations
- **THEN** the persisted failed precheck step includes a single rejection summary that mentions both blockers
- **AND** the summary uses deterministic wording and ordering so repeated reads show the same operator-visible reason

#### Scenario: Missing compute service preserves a readable precheck reason

- **WHEN** `slurm_to_openstack` precheck fails because the selected host has no readable OpenStack compute service
- **THEN** the persisted failed precheck step includes an operator-visible rejection summary describing that compute-service failure

#### Scenario: Generic precheck dependency failure still records the reason

- **WHEN** precheck fails before mutation because a required dependency or client is unavailable
- **THEN** the persisted failed precheck step includes a concise operator-visible rejection summary derived from that failure

#### Scenario: Failed placeholder job records state-based summary

- **WHEN** a `slurm_to_openstack` execution fails from `awaiting_source_allocation` because placeholder job `12345` entered Slurm state `FAILED`
- **THEN** the failed `wait_for_source_allocation` step includes an operator-visible summary mentioning job `12345` and state `FAILED`
- **AND** the execution detail also exposes that summary as its final failure reason

#### Scenario: Placeholder job completion without allocation records a readable reason

- **WHEN** a `slurm_to_openstack` execution fails from `awaiting_source_allocation` because its placeholder job finished before any allocation event was received
- **THEN** the failed `wait_for_source_allocation` step includes a readable summary explaining that allocation never arrived
- **AND** the execution detail exposes the same operator-visible reason

### Requirement: Failed attach steps preserve operator-visible reasons

The system SHALL persist an operator-visible error summary on a failed `attach_target` step when target attachment fails after the execution reaches `host_reachable`. The failed step MUST preserve the same underlying failure text used for the execution's terminal failure summary so the execution detail and step timeline remain consistent. This applies to both `slurm_to_openstack` and `openstack_to_slurm` attach failures.

#### Scenario: Slurm-to-OpenStack attach failure preserves the OpenStack error

- **WHEN** a `slurm_to_openstack` execution enters `host_reachable`
- **AND** enabling the target OpenStack compute service fails with an attach error
- **THEN** the persisted failed `attach_target` step includes an `error_summary` derived from that attach error
- **AND** the execution detail exposes the same failure summary

#### Scenario: OpenStack-to-Slurm attach failure preserves the Slurm readiness error

- **WHEN** an `openstack_to_slurm` execution enters `host_reachable`
- **AND** restoring Slurm attachment readiness fails with an attach error
- **THEN** the persisted failed `attach_target` step includes an `error_summary` derived from that attach error
- **AND** the execution detail exposes the same failure summary

#### Scenario: Missing attach dependency still records a readable failed step reason

- **WHEN** target attachment fails because a required attach dependency such as the OpenStack or Slurm client is not configured
- **THEN** the persisted failed `attach_target` step includes a readable `error_summary` derived from that dependency failure
- **AND** operators can inspect that reason from the execution step timeline without relying only on the execution-level error
