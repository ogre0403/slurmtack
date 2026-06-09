## ADDED Requirements

### Requirement: Blocked precheck steps preserve operator-visible reasons

The system SHALL persist an operator-visible rejection summary on failed switch steps when the daemon can identify a concrete blocker. For `openstack_to_slurm` precheck failures classified as `precheck_blocked`, the stored step record MUST preserve a deterministic summary that identifies the source-readiness blockers found during precheck, including resident instances and active migrations when present.

#### Scenario: Precheck records resident-instance blocker summary

- **WHEN** `openstack_to_slurm` precheck fails because the selected host still has resident instances
- **THEN** the persisted failed precheck step includes a stable rejection summary describing that resident-instance blocker

#### Scenario: Precheck records multiple blockers in stable order

- **WHEN** `openstack_to_slurm` precheck fails because the selected host has both resident instances and active migrations
- **THEN** the persisted failed precheck step includes a single rejection summary that mentions both blockers
- **AND** the summary uses deterministic wording and ordering so repeated reads show the same operator-visible reason
