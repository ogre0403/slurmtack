## 1. Config and request validation

- [x] 1.1 Update daemon config loading to add `PLACEHOLDER_SIF_FILE` and reinterpret `PLACEHOLDER_SIF_PATH` as a home-relative directory with validation for empty, absolute, or traversal values.
- [x] 1.2 Extend the switch request DTO/service request with optional `placeholder_sif_file` and resolve one effective filename from the API override or env default during `slurm_to_openstack` request intake.
- [x] 1.3 Reject `slurm_to_openstack` requests before execution creation when the effective placeholder SIF filename is missing or invalid, or when placeholder SIF path config is missing or invalid.

## 2. Execution persistence and async propagation

- [x] 2.1 Extend `domain.Execution`, SQLite schema/migrations, and store round-trip tests to persist the execution-scoped effective placeholder SIF filename.
- [x] 2.2 Thread the persisted placeholder SIF filename through orchestrator intake and Slurm placeholder request models so async submission and restart recovery use the same execution-scoped file selection.
- [x] 2.3 Add migration fallback so pre-change executions with an empty stored filename use the current `PLACEHOLDER_SIF_FILE` only for legacy compatibility.

## 3. Placeholder submission path resolution

- [x] 3.1 Update placeholder submission code in `internal/slurm/restclient.go` to resolve `/home/<workload-user>/<PLACEHOLDER_SIF_PATH>/<effective-file>` with normalized joining and shell-safe script generation.
- [x] 3.2 Keep placeholder job working, stdout, and stderr paths under `/home/<workload-user>` while reusing the resolved execution-scoped filename for the Singularity command.
- [x] 3.3 Update `docker/.env`, README, and related operator docs/examples to show the split path/file config and `placeholder_sif_file` API override behavior.

## 4. Verification

- [x] 4.1 Add or update API/service tests for default filename fallback, request-time filename override, invalid filename rejection, and missing placeholder config rejection.
- [x] 4.2 Add or update store/orchestrator/Slurm client tests for persisted filename round-trips, migration fallback, and resolved per-user SIF paths.
- [x] 4.3 Run focused tests for `internal/api`, `internal/service`, `internal/store`, `internal/orchestrator`, and `internal/slurm`.
