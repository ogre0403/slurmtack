## Why

The dashboard already requires operators to enter a placeholder SIF filename, but it does not tell them where that file must actually exist on disk for the derived Slurm workload user. Operators currently have to infer the resolved path from JWT claims and daemon `.env` configuration, which makes it easy to save a filename that points to the wrong home directory or relative SIF path.

## What Changes

- Show a read-only hint in the Slurm job settings UI that resolves the effective SIF file location from the derived workload username, the configured `SLURM_SIF_PATH`, and the typed `placeholder_sif_file`.
- Update the dashboard validation and empty states so the hint explains which part is still missing when the token cannot be decoded, the SIF filename is blank, or the daemon has no usable `SLURM_SIF_PATH` configured.
- Expose a safe dashboard-facing API contract that returns the configured home-relative `SLURM_SIF_PATH` metadata needed to build the operator-visible SIF location hint without exposing secrets.
- Keep switch-request payloads and runtime SIF resolution rules unchanged; this change improves operator confirmation before submission.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `node-switch-dashboard`: show the resolved SIF location hint in Slurm job settings and guide operators when the path cannot yet be computed.
- `rest-api`: expose non-secret dashboard metadata for the configured `SLURM_SIF_PATH` so the UI can assemble the effective per-user SIF path.

## Impact

- Affected code: `docker/nginx/html/index.html`, `docker/nginx/html/dashboard.js`, dashboard/API handler tests, and dashboard/operator documentation.
- Affected APIs: the dashboard will consume an authenticated read contract for `SLURM_SIF_PATH` metadata in addition to the existing switch and inventory flows.
- Affected operators: users configuring `slurm_to_openstack` from the dashboard get an explicit file-location hint before submitting a switch request.
