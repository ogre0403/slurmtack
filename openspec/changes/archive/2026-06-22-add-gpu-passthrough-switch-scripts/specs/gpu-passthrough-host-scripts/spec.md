## ADDED Requirements

### Requirement: Standalone GPU passthrough reconfiguration script

The system SHALL provide a standalone executable shell script for GPU passthrough reconfiguration that runs directly on a target node and accepts exactly one action argument: `enable` or `disable`. For `enable`, the script MUST discover NVIDIA PCI device IDs, ensure the passthrough IOMMU kernel arguments are configured, ensure VFIO boot/module configuration is written, and rebuild initramfs when a passthrough-related configuration change is made. For `disable`, the script MUST remove the passthrough-specific kernel arguments and VFIO configuration files, unload VFIO modules when possible before reboot, and rebuild initramfs when a passthrough-related configuration change is made. The script MUST exit non-zero for an unsupported action or when no NVIDIA GPU is detected for an `enable` action.

#### Scenario: Enable action prepares passthrough configuration
- **WHEN** an operator runs the reconfiguration script with `enable` on a node with NVIDIA GPUs
- **THEN** the script updates the host's passthrough-related kernel and VFIO configuration toward the enabled state
- **AND** the script exits successfully when the host is already configured or after applying the required changes

#### Scenario: Disable action removes passthrough configuration
- **WHEN** an operator runs the reconfiguration script with `disable`
- **THEN** the script removes passthrough-specific kernel and VFIO configuration from the host
- **AND** the script exits successfully when the host is already configured for the disabled state or after applying the required changes

#### Scenario: Unsupported action is rejected
- **WHEN** an operator runs the reconfiguration script with an action other than `enable` or `disable`
- **THEN** the script exits with a non-zero status and reports that the action is invalid

### Requirement: Standalone GPU passthrough verification script for enabled mode

The system SHALL provide a standalone executable shell script for GPU passthrough verification that accepts the action `enable` and validates the post-reboot enabled state. Successful verification MUST require the passthrough kernel arguments to be active in `/proc/cmdline`, VFIO modules to be loaded, passthrough-related VFIO configuration files to exist, and each detected NVIDIA GPU to report `Kernel driver in use: vfio-pci`. The script MUST exit non-zero when any of these checks fail.

#### Scenario: Enabled passthrough verifies successfully after reboot
- **WHEN** an operator runs the verification script with `enable` after a passthrough-enable reboot
- **THEN** the script exits successfully only if the node reports the expected enabled passthrough state

#### Scenario: Enabled passthrough verification fails on driver mismatch
- **WHEN** an operator runs the verification script with `enable`
- **AND** at least one detected NVIDIA GPU is not bound to `vfio-pci`
- **THEN** the script exits with a non-zero status and reports the binding mismatch

### Requirement: Standalone GPU passthrough verification script for disabled mode

The system SHALL provide a standalone executable shell script for GPU passthrough verification that accepts the action `disable` and validates the post-reboot disabled state. Successful verification MUST require the passthrough kernel arguments to be absent from `/proc/cmdline`, passthrough-related VFIO configuration files to be absent, and detected NVIDIA GPUs to no longer report `Kernel driver in use: vfio-pci`. The script MUST exit non-zero when any of these checks fail.

#### Scenario: Disabled passthrough verifies successfully after reboot
- **WHEN** an operator runs the verification script with `disable` after a passthrough-disable reboot
- **THEN** the script exits successfully only if the node reports the expected disabled passthrough state

#### Scenario: Disabled passthrough verification fails when vfio binding remains
- **WHEN** an operator runs the verification script with `disable`
- **AND** at least one detected NVIDIA GPU still reports `Kernel driver in use: vfio-pci`
- **THEN** the script exits with a non-zero status and reports that passthrough remains enabled
