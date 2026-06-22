# GPU Passthrough Scripts

Standalone, repo-owned shell scripts that put a GPU node into (or out of) VFIO
passthrough state. They replace the former `hack/gpu-passthrough` Ansible role
and are the same artifacts the switch orchestrator stages and runs over SSH.

## Layout

| File              | Purpose                                                           |
| ----------------- | ----------------------------------------------------------------- |
| `lib.sh`          | Shared primitives (NVIDIA PCI detection, VFIO paths, checks).     |
| `reconfigure.sh`  | Apply (`enable`) or remove (`disable`) passthrough configuration. |
| `verify.sh`       | Validate the post-reboot `enable` / `disable` state.              |
| `test_scripts.sh` | Sandbox tests that stub host tools; run without a GPU.            |

`reconfigure.sh` and `verify.sh` each take exactly one action argument,
`enable` or `disable`, and exit non-zero on any other value.

## CLI

```shell
# Prepare a node for passthrough (writes VFIO config, IOMMU kernel args,
# rebuilds initramfs). Does NOT reboot.
sudo ./reconfigure.sh enable

# Remove passthrough configuration. Does NOT reboot.
sudo ./reconfigure.sh disable

# After rebooting, confirm the resulting state.
sudo ./verify.sh enable      # requires IOMMU args, VFIO modules + config, vfio-pci binding
sudo ./verify.sh disable     # requires args/config absent and no vfio-pci binding
```

`reconfigure.sh` must run as root because it mutates boot configuration and
rebuilds initramfs. The script is idempotent: a second `enable`/`disable` run on
an already-converged host makes no changes and skips the initramfs rebuild.

## Verification contract

- **enable** passes only when every passthrough kernel arg is active in
  `/proc/cmdline`, the VFIO modules are loaded, both VFIO config files exist, and
  every detected NVIDIA device reports `Kernel driver in use: vfio-pci`.
- **disable** passes only when the passthrough kernel args are absent, both VFIO
  config files are gone, and no detected NVIDIA device is still bound to
  `vfio-pci`. A leftover binding or config file fails verification.

## Pre-integration validation on target nodes

Validate the scripts directly on a representative GPU node before turning on the
orchestrator integration (`GPU_PASSTHROUGH_SCRIPT_DIR`):

1. Copy this directory to the node (e.g. `scp -r scripts/gpu-passthrough …`).
2. Run `sudo ./reconfigure.sh enable`, then reboot.
3. Run `sudo ./verify.sh enable` and confirm it exits `0`.
4. Run `sudo ./reconfigure.sh disable`, then reboot.
5. Run `sudo ./verify.sh disable` and confirm it exits `0`.

Only after both directions verify cleanly should the daemon be pointed at the
scripts.

## Non-GPU test environments

For CI or dev hosts without NVIDIA GPUs, use `scripts/fake-passthrough` instead.
It exposes the same `reconfigure.sh` / `verify.sh` interface and succeeds as a
deterministic no-op on any host, letting you validate the full orchestration path
(scp staging, SSH execution, direction mapping) without real GPU hardware.

```shell
# Point at the fake bundle for non-GPU testing only.
GPU_PASSTHROUGH_SCRIPT_DIR=/path/to/repo/scripts/fake-passthrough
```

See `scripts/fake-passthrough/README.md` for details and the safety warning.

## Orchestrator integration

When `GPU_PASSTHROUGH_SCRIPT_DIR` is set to a compatible bundle directory, the switch daemon:

- before reboot, stages `lib.sh` + `reconfigure.sh` into an execution-scoped
  remote directory (`${REMOTE_STAGING_DIR:-/tmp}/slurmtack-gpu-passthrough/<execution-id>`)
  with `scp` and runs `reconfigure.sh` over SSH — `enable` for
  `slurm_to_openstack`, `disable` for `openstack_to_slurm`;
- after the node is SSH-reachable again and before any target attach action,
  stages `lib.sh` + `verify.sh` and runs `verify.sh` in the same mode.

A staging or non-zero script result fails the execution before it proceeds.
Leaving `GPU_PASSTHROUGH_SCRIPT_DIR` unset disables this integration so the
scripts can still be exercised manually.

## Tests

```shell
./test_scripts.sh
```

The tests stub `lspci`, `grubby`, `dracut`, `lsmod`, `modprobe`, and `id` on
`PATH` and redirect the VFIO config and `/proc/cmdline` paths into a sandbox, so
they run on any host without real GPUs.
