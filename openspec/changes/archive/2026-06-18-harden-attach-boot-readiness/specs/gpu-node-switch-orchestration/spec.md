## MODIFIED Requirements

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
