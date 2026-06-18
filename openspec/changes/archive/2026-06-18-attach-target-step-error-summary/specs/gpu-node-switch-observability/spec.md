## ADDED Requirements

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
