## Why

The daemon, docs, and specs still expose placeholder-oriented environment names for the Slurm SIF image configuration. Those names no longer match their operational purpose, so operators have to learn an outdated placeholder concept even though the values are now part of the Slurm-side runtime contract.

## What Changes

- Rename the daemon environment variables `PLACEHOLDER_SIF_PATH` and `PLACEHOLDER_SIF_FILE` to `SLURM_SIF_PATH` and `SLURM_SIF_FILE`.
- Update runtime validation, request-time error messages, and job submission behavior to resolve the SIF image from the renamed environment variables without changing the current home-relative path semantics.
- Update deployment and API-facing documentation so operators configure and troubleshoot the renamed variables consistently.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `daemon-deployment`: rename the documented and validated SIF-related environment variables in the daemon configuration contract.
- `placeholder-agent-packaging`: rename the environment variables used to describe how placeholder-job SIF images are deployed under workload user home directories.
- `slurmrestd-client`: rename the environment variables used when resolving the per-user SIF image path for Slurm job submission.
- `rest-api`: rename the environment variable referenced by request validation failures when placeholder SIF configuration is missing or invalid.

## Impact

- Affected code: environment loading and validation in `internal/config`, request validation and error text in `internal/service`, runtime SIF path resolution for Slurm submission, and README / deployment examples.
- Affected operators: existing deployments must rename the two environment variables in daemon configuration.
- No new external dependencies or new runtime capabilities are introduced.
