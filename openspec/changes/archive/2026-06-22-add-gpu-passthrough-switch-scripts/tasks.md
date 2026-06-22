## 1. Standalone GPU Passthrough Scripts

- [x] 1.1 Create a repo-owned GPU passthrough script directory with a stable CLI for `reconfigure <enable|disable>` and `verify <enable|disable>`.
- [x] 1.2 Port the current `hack/gpu-passthrough` enable behavior into the reconfiguration and verification scripts, including NVIDIA PCI ID detection, VFIO file management, initramfs rebuild, and enabled-state validation.
- [x] 1.3 Port the current disable behavior into the reconfiguration script and add the missing disabled-state verification checks for post-reboot validation.
- [x] 1.4 Add direct script-level tests or validation fixtures so the shell scripts can be exercised independently of the orchestrator.

## 2. Remote Staging Support

- [x] 2.1 Extend the remote execution layer with an `scp`-backed file staging operation that reuses the configured SSH user, port, key, and SSH options.
- [x] 2.2 Add tests for remote staging command rendering, transport reuse, and failure propagation.
- [x] 2.3 Add an orchestrator helper that stages a local script into an execution-scoped remote path and executes it over SSH with explicit step names and timeouts.

## 3. Workflow Integration

- [x] 3.1 Update the `slurm_to_openstack` host reconfiguration flow to stage and execute the GPU passthrough reconfiguration script in `enable` mode before reboot.
- [x] 3.2 Update the `openstack_to_slurm` host reconfiguration flow to stage and execute the GPU passthrough reconfiguration script in `disable` mode before reboot.
- [x] 3.3 Update the post-reboot pre-attach flow so `slurm_to_openstack` stages and runs the GPU passthrough verification script in `enable` mode before OpenStack attach actions.
- [x] 3.4 Update the post-reboot pre-attach flow so `openstack_to_slurm` stages and runs the GPU passthrough verification script in `disable` mode before `slurmd` restore and Slurm attach checks.

## 4. Regression Coverage And Operator Validation

- [x] 4.1 Add orchestrator tests covering mode selection, `scp`/SSH ordering, and failure handling for both reconfiguration and verification steps in both switch directions.
- [x] 4.2 Add coverage for the disabled-state verification contract so a leftover `vfio-pci` binding or passthrough config fails the workflow.
- [x] 4.3 Update operator-facing documentation for the standalone script workflow and record the expected pre-integration validation steps on target nodes.
