## 1. Add the fake passthrough script bundle

- [x] 1.1 Create `scripts/fake-passthrough` with the compatible entrypoints and helper files expected by the existing passthrough staging flow.
- [x] 1.2 Implement fake `reconfigure.sh` and `verify.sh` so `enable` and `disable` both complete as deterministic test-only no-ops, while invalid actions still fail clearly.
- [x] 1.3 Add script-level coverage proving the fake bundle preserves the `gpu-passthrough` CLI surface and succeeds on hosts without GPUs.

## 2. Keep orchestration compatibility through the existing configuration surface

- [x] 2.1 Update or extend passthrough staging/execution tests so `GPU_PASSTHROUGH_SCRIPT_DIR` can point at the fake bundle without adding orchestration-specific branching.
- [x] 2.2 Keep the existing direction mapping intact so `slurm_to_openstack` still runs `enable` and `openstack_to_slurm` still runs `disable` when the fake bundle is selected for reconfigure and verify.

## 3. Document and validate non-GPU workflow testing

- [x] 3.1 Update operator-facing docs and config examples to explain when to use `scripts/gpu-passthrough` versus `scripts/fake-passthrough`, and mark the fake bundle as test-only.
- [x] 3.2 Run the focused script and orchestrator tests needed to confirm the fake bundle enables end-to-end workflow validation in non-GPU environments without weakening the real passthrough bundle contract.
