## Why

The switch workflow already uses the Slurm API to drain and resume nodes, but it does not explicitly manage the `slurmd` service during ownership handoff. That leaves a gap where a node can keep advertising itself to Slurm after drain or remain unavailable after OpenStack handoff because `slurmd` was not re-enabled before `RESUME`.

## What Changes

- Add an explicit `slurmd` shutdown step in the `slurm_to_openstack` flow after the node is drained through the Slurm API and before host reconfiguration continues.
- Add an explicit `slurmd` startup step in the `openstack_to_slurm` flow before the daemon issues `ResumeNode` through the Slurm API.
- Require the daemon to execute `systemctl stop slurmd` and `systemctl disable slurmd` through the SSH runner when handing a node from Slurm to OpenStack.
- Require the daemon to execute `systemctl enable slurmd` and `systemctl start slurmd` through the SSH runner when handing a node from OpenStack back to Slurm.
- Extend workflow validation and tests so service-control failures block the state transition instead of allowing the switch to continue with partial Slurm availability.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `gpu-node-switch-orchestration`: tighten the handoff sequence so `slurmd` is stopped and disabled after Slurm drain, and enabled and started before Slurm resume.

## Impact

- Affected code: switch orchestration steps, SSH-runner-backed host mutation helpers, and workflow tests covering Slurm and OpenStack handoff ordering.
- Affected systems: GPU nodes managed by `slurmd`, Slurm control-plane interactions, and OpenStack-to-Slurm / Slurm-to-OpenStack handoff safety.
- Operational impact: reduces the risk of stale Slurm heartbeats during detach and ensures nodes rejoin Slurm only after `slurmd` is restored.