## 1. Rename the daemon SIF environment contract

- [x] 1.1 Update config loading and validation to read `SLURM_SIF_PATH` / `SLURM_SIF_FILE` and emit operator-facing validation errors with the new env names.
- [x] 1.2 Update service and Slurm client wiring to use the renamed env-backed config values while keeping `placeholder_sif_file` request behavior unchanged.

## 2. Refresh tests for the renamed envs

- [x] 2.1 Update unit and integration tests that assert SIF env loading, validation failures, request-time fallback behavior, and Slurm job path resolution to reference `SLURM_SIF_PATH` / `SLURM_SIF_FILE`.
- [x] 2.2 Add or adjust coverage for missing-config error messages so operator guidance points to `SLURM_SIF_FILE` and `SLURM_SIF_PATH`.

## 3. Update operator documentation and deployment examples

- [x] 3.1 Update README and any deployment/env examples to document `SLURM_SIF_PATH` / `SLURM_SIF_FILE`, including direct migration guidance from the old names.
- [x] 3.2 Update build and packaging guidance that currently prints or describes `PLACEHOLDER_SIF_*` so all operator-facing output uses the new env names consistently.
