## Context

`docs/switch-design.md` already treats GPU passthrough mutation as a required part of the host reconfiguration and verification phases, but the repository only contains that logic under `hack/` as notes plus an Ansible role. The current orchestrator implementation also has no GPU passthrough step yet: `doReconfigure` only quiesces `slurmd` for `slurm_to_openstack`, `doAttach` only restores `slurmd` or enables OpenStack services, and `internal/remote` only exposes SSH command execution, not file staging.

The `hack/gpu-passthrough` role gives us the intended host behavior:
- passthrough enable: discover NVIDIA PCI IDs, add IOMMU kernel args, write VFIO module and modprobe files, rebuild initramfs, reboot, then verify VFIO binding after reboot
- passthrough disable: remove VFIO-related files and kernel args, rebuild initramfs, reboot, but currently without an explicit post-reboot verification contract

The requested workflow adds one operational constraint on top of that host behavior: both reconfiguration and verification must first copy the script to the target node with `scp`, then execute it remotely over SSH. The scripts must be independently runnable before the orchestrator depends on them.

## Goals / Non-Goals

**Goals:**
- Replace the `hack/gpu-passthrough` Ansible role with standalone executable shell scripts checked into the repo.
- Define one consistent passthrough contract for both directions: `slurm_to_openstack` enables passthrough before reboot and verifies enabled state after reboot; `openstack_to_slurm` disables passthrough before reboot and verifies disabled state after reboot.
- Integrate script staging and execution into the existing switch workflow without adding a new top-level state machine.
- Reuse the existing SSH identity, port, key, and options for both `scp` and SSH execution.

**Non-Goals:**
- Redesign the overall switch state model or replace SSH-based host mutation with another transport.
- Install GPU drivers, CUDA packages, or other non-passthrough host software.
- Fold every future host mutation into a generic remote configuration framework in this change.

## Decisions

### Use two mode-parameterized scripts instead of direction-specific copies

The repo will introduce a small GPU passthrough script set with a stable CLI surface:
- a reconfiguration script that accepts `enable` or `disable`
- a verification script that accepts `enable` or `disable`

This keeps the standalone/manual workflow simple while avoiding duplicated detection and validation logic between the two directions. The main shared primitives are NVIDIA PCI ID discovery, kernel-argument inspection, VFIO file paths, and driver-binding checks.

Alternative considered: four separate scripts (`enable`, `disable`, `verify-enable`, `verify-disable`).
Rejected because it duplicates too much logic and makes orchestrator integration branch on filenames instead of only on the requested passthrough mode.

### Make post-reboot verification explicit for both passthrough modes

The script contract will treat verification as a first-class operation rather than an informal follow-up command sequence. For `enable`, verification will require the IOMMU args to be active, VFIO boot configuration to be present, VFIO modules to be loaded, and detected NVIDIA devices to be bound to `vfio-pci`. For `disable`, verification will require the passthrough-specific kernel args and VFIO config files to be absent and the detected NVIDIA devices to no longer be bound to `vfio-pci`.

The disable path intentionally verifies removal of passthrough state rather than requiring a specific replacement driver. That keeps the contract aligned with the current `hack/` material, which removes passthrough configuration but does not itself manage the full OpenStack-side NVIDIA driver stack.

Alternative considered: keep verification only for the enable path.
Rejected because the workflow needs a symmetric success criterion after reboot in both directions, and because disabling passthrough without a post-reboot check can hide a partial rollback.

### Integrate scripts at existing reconfigure and host-reachable boundaries

The orchestrator will keep the current high-level progression:
- source detach / host reconfigure
- reboot / SSH reachability
- target attach
- target verify

GPU passthrough script execution fits into that model without adding a new persisted state:
- before reboot, during host reconfiguration, stage the reconfiguration script with `scp` and execute it over SSH in the direction-appropriate mode
- after reboot, once SSH reachability is satisfied and before target-side attach actions begin, stage the verification script with `scp` and execute it over SSH in the same mode

Verifying passthrough before target attachment is deliberate. If the node comes back with the wrong host configuration, the workflow should fail before it resumes Slurm service or re-enables OpenStack compute ownership.

Alternative considered: perform passthrough verification inside the later `verifying` phase.
Rejected because that would defer a host-configuration failure until after more target-side mutation has already happened.

### Add explicit remote file staging support that reuses SSH configuration

The remote layer will gain an explicit staging/copy operation backed by `scp`, using the same SSH user, port, private key, and options already configured for reboot and reachability. The orchestrator will stage scripts into an execution-scoped temporary directory on the target node, then execute them via SSH by absolute path.

This preserves the requirement that the same artifact validated manually is the one used in orchestration, and it avoids embedding large shell payloads inline inside SSH commands.

Alternative considered: inline the shell body directly in the remote SSH command.
Rejected because it breaks the “validate the script first, then integrate it” requirement and makes local script testing diverge from runtime behavior.

### Keep standalone validation as the first delivery milestone

Implementation will land the shell scripts and their direct validation path before wiring them into the end-to-end switch workflow. That sequencing reduces integration risk: the daemon will only start depending on the scripts after their enable/disable/verify behavior has been exercised on representative nodes.

Alternative considered: land script integration and script creation in one step.
Rejected because it would make it harder to distinguish host-script defects from orchestration defects during bring-up.

## Risks / Trade-offs

- [Disable verification may be too weak if operations expect a specific NVIDIA driver state after reboot] → Mitigation: define the normative contract around removal of passthrough state and non-`vfio-pci` binding, and tighten it later only if the OpenStack-side driver baseline becomes explicit.
- [Adding `scp` broadens the remote execution surface] → Mitigation: reuse the same SSH transport settings, keep the copied payload limited to repo-owned executable scripts, and scope remote staging to a temporary execution directory.
- [A failed reconfiguration script can leave partial local host config before reboot] → Mitigation: keep failures in the existing mutation-partial path and make script stderr/stdout visible enough for operator recovery.
- [A reboot can clear the staged artifact directory depending on remote path choice] → Mitigation: treat pre-reboot staging and post-reboot staging as separate operations and re-copy the verify script after reachability is restored.

## Migration Plan

1. Add the standalone GPU passthrough shell scripts and direct script-level tests.
2. Validate the scripts against the existing `hack/` expectations for enable and disable behavior, including post-reboot verification checks.
3. Extend the remote package with `scp`-backed staging support that shares existing SSH configuration.
4. Wire the reconfiguration script into `doReconfigure` and the verification script into the post-reboot pre-attach path.
5. Add orchestrator coverage for both switch directions and failure paths.

Rollback is a code revert of the script integration. Because no persisted state or API contract changes are required, rollback does not need a data migration.

## Open Questions

- Which remote staging directory is safest across supported node images: an execution-specific directory under `/tmp`, `/var/tmp`, or another operator-approved path?
- Should the verify script emit only pass/fail diagnostics, or also structured machine-readable output for future evidence capture?
