## ADDED Requirements

### Requirement: Slurmd is quiesced before Slurm-to-OpenStack host mutation

For `slurm_to_openstack` executions, after the node has been drained through the Slurm API and before the daemon proceeds with host-side reconfiguration, the system SHALL use the configured SSH runner to execute `systemctl stop slurmd` and then `systemctl disable slurmd` on the target node. If either command fails, the workflow MUST fail and MUST NOT continue to host mutation or target attachment.

#### Scenario: Drained node stops and disables slurmd before reconfiguration
- **WHEN** a `slurm_to_openstack` execution has completed its Slurm drain workflow and is ready to leave source ownership
- **THEN** the daemon executes `systemctl stop slurmd` followed by `systemctl disable slurmd` through the SSH runner before transitioning deeper into host reconfiguration

#### Scenario: Slurmd shutdown failure blocks the handoff
- **WHEN** a `slurm_to_openstack` execution cannot stop or disable `slurmd` through the SSH runner
- **THEN** the daemon records the step as failed and does not continue to host mutation or OpenStack attachment

### Requirement: Slurmd is restored before OpenStack-to-Slurm attachment

For `openstack_to_slurm` executions, before the daemon evaluates Slurm attach readiness or issues `ResumeNode`, the system SHALL use the configured SSH runner to execute `systemctl enable slurmd` and then `systemctl start slurmd` on the target node. If either command fails, the workflow MUST fail and MUST NOT issue `ResumeNode` or declare the node ready for Slurm attachment.

#### Scenario: Slurmd enable and start happen before Slurm attach evaluation
- **WHEN** an `openstack_to_slurm` execution reaches target attachment after host reconfiguration
- **THEN** the daemon executes `systemctl enable slurmd` followed by `systemctl start slurmd` through the SSH runner before it evaluates node attach state or calls the Slurm resume API

#### Scenario: Slurmd restore failure blocks Slurm re-entry
- **WHEN** an `openstack_to_slurm` execution cannot enable or start `slurmd` through the SSH runner
- **THEN** the daemon records the step as failed and does not issue `ResumeNode` or complete Slurm attachment