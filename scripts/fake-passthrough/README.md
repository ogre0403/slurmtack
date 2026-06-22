# Fake GPU Passthrough Scripts — TEST-ONLY

> **WARNING:** This bundle is for non-GPU test and CI environments only. It does
> not interact with GPU hardware, VFIO, kernel arguments, or initramfs. Do **not**
> point a production daemon at this directory.

Fake equivalents of `scripts/gpu-passthrough` that expose the same filenames and
CLI interface (`reconfigure.sh <enable|disable>`, `verify.sh <enable|disable>`)
while succeeding as deterministic no-ops on any host, including hosts without
NVIDIA GPUs.

## Purpose

The real `scripts/gpu-passthrough` bundle correctly fails on hosts without GPUs.
In non-GPU test or CI environments this makes it impossible to tell whether the
orchestration flow (staging, SSH execution, post-reboot verify, direction mapping)
is working correctly, because the host hardware check always fails first.

This fake bundle lets you validate the full switch orchestration path — scp
staging, SSH execution ordering, reboot boundary, direction-specific mode
selection — without a GPU present.

## Layout

| File              | Purpose                                                            |
| ----------------- | ------------------------------------------------------------------ |
| `lib.sh`          | Shared log/fail helpers (no hardware calls).                       |
| `reconfigure.sh`  | Fake reconfiguration — no-op success for `enable` and `disable`.  |
| `verify.sh`       | Fake verification — no-op success for `enable` and `disable`.     |
| `test_scripts.sh` | Script-level tests for the fake bundle.                            |

## CLI

```shell
# Both exit zero and print a [FAKE] label to stderr.
./reconfigure.sh enable
./reconfigure.sh disable
./verify.sh enable
./verify.sh disable

# Both exit non-zero for unsupported actions (same contract as the real bundle).
./reconfigure.sh bogus   # exit 1
./verify.sh bogus        # exit 1
```

## Selecting the fake bundle

Set `GPU_PASSTHROUGH_SCRIPT_DIR` to this directory instead of
`scripts/gpu-passthrough`. No other orchestrator configuration changes are needed:

```shell
# Non-GPU test environment
GPU_PASSTHROUGH_SCRIPT_DIR=/path/to/repo/scripts/fake-passthrough

# Production GPU nodes (real bundle — the default)
GPU_PASSTHROUGH_SCRIPT_DIR=/path/to/repo/scripts/gpu-passthrough
```

Rollback is a single config change back to the real bundle path.

## When to use each bundle

| Bundle                    | Use when                                              |
| ------------------------- | ----------------------------------------------------- |
| `scripts/gpu-passthrough` | Production GPU nodes; any host where VFIO is needed.  |
| `scripts/fake-passthrough`| Non-GPU CI/test environments; orchestration testing.  |

## Tests

```shell
./test_scripts.sh
```
