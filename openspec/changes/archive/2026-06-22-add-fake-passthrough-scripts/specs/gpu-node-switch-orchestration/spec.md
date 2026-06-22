## MODIFIED Requirements

### Requirement: GPU passthrough reconfiguration is staged and executed before reboot

For both switch directions, before the workflow reboots the host, the system SHALL copy the configured passthrough reconfiguration script bundle to the target node with `scp` and then execute the staged script over SSH. The configured bundle MUST implement the same `reconfigure.sh <enable|disable>` interface as the repo-owned passthrough bundles, and the copy and execution steps MUST use the same configured SSH identity, port, private key, and SSH options already used for reboot and reachability operations. For `slurm_to_openstack`, the script MUST run in `enable` mode. For `openstack_to_slurm`, the script MUST run in `disable` mode. If staging or execution fails, the workflow MUST fail and MUST NOT proceed to reboot.

#### Scenario: Slurm-to-OpenStack stages and runs passthrough enable before reboot
- **WHEN** a `slurm_to_openstack` execution reaches host reconfiguration after source detachment
- **THEN** the daemon copies the passthrough reconfiguration script to the node with `scp`
- **AND** the daemon executes the staged script over SSH with the `enable` action before any reboot is triggered

#### Scenario: OpenStack-to-Slurm stages and runs passthrough disable before reboot
- **WHEN** an `openstack_to_slurm` execution reaches host reconfiguration after source detachment
- **THEN** the daemon copies the passthrough reconfiguration script to the node with `scp`
- **AND** the daemon executes the staged script over SSH with the `disable` action before any reboot is triggered

#### Scenario: Compatible fake bundle uses the same reconfigure orchestration path
- **WHEN** `GPU_PASSTHROUGH_SCRIPT_DIR` points at the fake passthrough bundle in a non-GPU test environment
- **AND** a switch execution reaches host reconfiguration
- **THEN** the daemon stages `reconfigure.sh` from that configured bundle
- **AND** the daemon executes it with the same direction-specific `enable` or `disable` action used for the real passthrough bundle

#### Scenario: Reconfiguration staging failure blocks reboot
- **WHEN** the daemon cannot copy the passthrough reconfiguration script to the node or the staged script exits non-zero
- **THEN** the workflow records the host reconfiguration step as failed
- **AND** the execution does not transition into `rebooting`

### Requirement: GPU passthrough verification is staged and executed after reboot before target attach

After a rebooted node becomes SSH-reachable and before the workflow performs target-side attach actions, the system SHALL copy the configured passthrough verification script bundle to the target node with `scp` and then execute the staged script over SSH. The configured bundle MUST implement the same `verify.sh <enable|disable>` interface as the repo-owned passthrough bundles. For `slurm_to_openstack`, the verification script MUST run in `enable` mode before the daemon enables OpenStack compute ownership. For `openstack_to_slurm`, the verification script MUST run in `disable` mode before the daemon restores `slurmd` or evaluates Slurm attach readiness. If staging or execution fails, the workflow MUST fail and MUST NOT continue to target attachment.

#### Scenario: Slurm-to-OpenStack verifies enabled passthrough before OpenStack attach
- **WHEN** a `slurm_to_openstack` execution reaches `host_reachable` after reboot
- **THEN** the daemon copies the passthrough verification script to the node with `scp`
- **AND** the daemon executes the staged script over SSH with the `enable` action before enabling the OpenStack compute service

#### Scenario: OpenStack-to-Slurm verifies disabled passthrough before Slurm attach
- **WHEN** an `openstack_to_slurm` execution reaches `host_reachable` after reboot
- **THEN** the daemon copies the passthrough verification script to the node with `scp`
- **AND** the daemon executes the staged script over SSH with the `disable` action before enabling `slurmd`, starting `slurmd`, or evaluating Slurm attach readiness

#### Scenario: Compatible fake bundle uses the same post-reboot verify path
- **WHEN** `GPU_PASSTHROUGH_SCRIPT_DIR` points at the fake passthrough bundle in a non-GPU test environment
- **AND** a switch execution reaches the post-reboot verification point
- **THEN** the daemon stages `verify.sh` from that configured bundle
- **AND** the daemon executes it with the same direction-specific `enable` or `disable` action used for the real passthrough bundle before target attach actions continue

#### Scenario: Verification failure blocks target attachment
- **WHEN** the daemon cannot copy the passthrough verification script to the node or the staged verification script exits non-zero
- **THEN** the workflow records the post-reboot host verification as failed
- **AND** the execution does not perform target attachment actions for that switch direction
