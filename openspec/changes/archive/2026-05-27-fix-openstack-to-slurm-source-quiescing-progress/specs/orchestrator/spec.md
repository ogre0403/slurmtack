## ADDED Requirements

### Requirement: OpenStack-to-Slurm source quiesce verification

The orchestrator SHALL actively re-evaluate `openstack_to_slurm` executions in `source_quiescing` on each tick. It MUST verify that the host's OpenStack compute service is disabled and that the host has no resident instances or active migrations before transitioning the execution to `source_detached`.

#### Scenario: Source quiesce still in progress
- **WHEN** an `openstack_to_slurm` execution is in `source_quiescing` and the compute service is still enabled, or instances or active migrations are still present on the host
- **THEN** the orchestrator leaves the execution in `source_quiescing` and retries verification on a later tick

#### Scenario: Source quiesce verification succeeds
- **WHEN** an `openstack_to_slurm` execution is in `source_quiescing`, the compute service is disabled, and the host has no resident instances or active migrations
- **THEN** the orchestrator transitions the execution to `source_detached`

#### Scenario: Source quiesce verification query fails
- **WHEN** an `openstack_to_slurm` execution is in `source_quiescing` and the orchestrator cannot read the required OpenStack quiesce signals
- **THEN** the verification action fails and the orchestrator applies its normal step-failure handling for that execution