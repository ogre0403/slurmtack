## ADDED Requirements

### Requirement: OpenStack-to-Slurm precheck gates source readiness

For `openstack_to_slurm` executions in `locked`, the orchestrator SHALL verify source readiness before it transitions the execution to `precheck_passed` or starts source quiesce. This precheck MUST confirm that the selected host has no resident instances and no active migrations, and it MUST still verify that the OpenStack compute service can be queried for that host. If any readiness blocker is present, the execution MUST fail as `precheck_blocked` from the precheck step and MUST NOT enter `source_quiescing`.

#### Scenario: Resident instances block switching during precheck

- **WHEN** an `openstack_to_slurm` execution is in `locked`
- **AND** precheck finds one or more resident instances on the selected host
- **THEN** the precheck step fails with failure class `precheck_blocked`
- **AND** the execution does not transition to `precheck_passed` or `source_quiescing`

#### Scenario: Active migrations block switching during precheck

- **WHEN** an `openstack_to_slurm` execution is in `locked`
- **AND** precheck finds one or more active migrations on the selected host
- **THEN** the precheck step fails with failure class `precheck_blocked`
- **AND** the execution does not transition to `precheck_passed` or `source_quiescing`

#### Scenario: Cleared source host passes precheck

- **WHEN** an `openstack_to_slurm` execution is in `locked`
- **AND** precheck can read the host's compute service
- **AND** the host has no resident instances or active migrations
- **THEN** the precheck step succeeds
- **AND** the execution transitions to `precheck_passed`

### Requirement: Slurm-to-OpenStack precheck validates target compute readiness

For `slurm_to_openstack` executions in `locked`, the orchestrator SHALL verify that the selected host exposes a readable OpenStack compute service before it transitions the execution to `precheck_passed` or starts source quiesce. If the compute service is missing or cannot be queried, the execution MUST fail as `precheck_blocked` from the precheck step and MUST NOT enter `precheck_passed` or `source_quiescing`.

#### Scenario: Missing compute service blocks switching during precheck

- **WHEN** a `slurm_to_openstack` execution is in `locked`
- **AND** precheck cannot find a compute service for the selected host
- **THEN** the precheck step fails with failure class `precheck_blocked`
- **AND** the execution does not transition to `precheck_passed` or `source_quiescing`

#### Scenario: Reachable compute service allows precheck to pass

- **WHEN** a `slurm_to_openstack` execution is in `locked`
- **AND** precheck can query the selected host's compute service
- **THEN** the precheck step succeeds
- **AND** the execution transitions to `precheck_passed`
