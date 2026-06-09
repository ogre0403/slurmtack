## MODIFIED Requirements

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
