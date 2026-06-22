## Why

The switch workflow already models host reconfiguration, reboot, and post-reboot verification, but the GPU passthrough work still lives only as `hack/` notes and Ansible tasks. We need a repo-native, executable shell-script contract that can be validated independently first and then integrated into the `openstack_to_slurm` and `slurm_to_openstack` paths without leaving the passthrough behavior implicit.

## What Changes

- Add standalone shell scripts that replace the current `hack/gpu-passthrough` Ansible role for GPU passthrough enable, disable, and verification behavior.
- Define a consistent verification contract for both passthrough directions, including the currently missing post-reboot verification path for passthrough disable.
- Update the node-switch orchestration requirements so host reconfiguration stages the required script onto the target node with `scp`, executes it over SSH before reboot, and runs the corresponding verify script over SSH after reboot.
- Keep script development and validation decoupled from immediate workflow integration so the scripts can be exercised directly on target nodes before the orchestrator starts depending on them.

## Capabilities

### New Capabilities
- `gpu-passthrough-host-scripts`: standalone shell scripts for enabling, disabling, and verifying GPU passthrough configuration on a node.

### Modified Capabilities
- `gpu-node-switch-orchestration`: the host reconfiguration and post-reboot verification sequence now stages and executes GPU passthrough scripts through `scp` and SSH in both switch directions.

## Impact

- Affected systems: GPU node host configuration, reboot-time handoff between Slurm and OpenStack, SSH-based remote execution, and post-reboot verification flow.
- Affected code: orchestrator host reconfiguration / attach / verify flow, remote execution support, packaging or repo layout for executable scripts, and tests covering both switch directions.
- Operational impact: operators gain a directly runnable script path for GPU passthrough changes before the daemon integration is turned on.
