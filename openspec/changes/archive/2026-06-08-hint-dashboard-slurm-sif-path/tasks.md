## 1. Dashboard settings metadata API

- [x] 1.1 Add an authenticated `GET /v1/dashboard/settings` handler and response model that expose `slurm_sif_path_configured` plus the configured home-relative `SLURM_SIF_PATH` without leaking credentials or derived-user data.
- [x] 1.2 Add or update API tests to cover both configured and unconfigured `SLURM_SIF_PATH` responses for the dashboard settings endpoint.

## 2. Dashboard SIF location hint

- [x] 2.1 Extend dashboard state and startup flow to load dashboard settings metadata, store the `SLURM_SIF_PATH` hint inputs, and recompute the expected SIF location whenever the Slurm token or SIF filename changes.
- [x] 2.2 Update the Slurm settings panel UI to show the expected SIF location as `/home/<derived-user>/<SLURM_SIF_PATH>/<placeholder_sif_file>` when resolvable and show explicit guidance when the user, filename, or daemon path config is missing.
- [x] 2.3 Extend dashboard HTML/JS tests to cover the new settings metadata fetch, resolved path rendering, and missing-path guidance states.

## 3. Documentation

- [x] 3.1 Update dashboard/operator documentation to explain that the UI shows the expected per-user SIF location derived from the JWT workload user and daemon `SLURM_SIF_PATH`, and that the hint is guidance rather than a live filesystem existence check.
