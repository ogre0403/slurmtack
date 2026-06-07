## Why

The archived June 7 changes already let `slurm_to_openstack` requests carry `slurm_account`, `slurm_user`, `slurm_user_token`, and `placeholder_sif_file`, but the dashboard still cannot collect or send those values. Operators using the UI currently have no practical way to configure the placeholder-job submission profile, and the dashboard does not yet enforce that this Slurm information is complete before launching a switch.

## What Changes

- Add a dashboard control in the top-right header area that opens Slurm job settings for `slurm_user_token`, `slurm_account`, and `placeholder_sif_file`.
- Persist the Slurm job settings in browser storage so the operator does not need to re-enter them on every page load.
- Decode the configured Slurm JWT in the browser to derive the effective `slurm_user`, display that derived username as read-only operator feedback, and send it together with the token on `slurm_to_openstack` requests.
- Require a complete Slurm job settings profile before the dashboard allows `slurm_to_openstack` to start; if token, derived user, account, or SIF filename is missing or invalid, the UI blocks the switch and shows the reason.
- Leave `openstack_to_slurm`, inventory reads, and history flows unchanged.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `node-switch-dashboard`: add a header-level Slurm job settings UI, client-side JWT-derived workload user behavior, browser persistence, and `slurm_to_openstack` request gating/composition that requires completed Slurm submission settings.

## Impact

- Affected code: `docker/nginx/html/index.html`, `docker/nginx/html/dashboard.js`, `internal/api/dashboard_ui_test.go`, and `docs/dashboard.md`.
- Affected APIs: no new endpoints or backend contract changes; the dashboard will require and use the existing `slurm_account`, `placeholder_sif_file`, `slurm_user`, and `slurm_user_token` request fields for dashboard-triggered `slurm_to_openstack`.
- Affected systems: browser `localStorage` and the operator workflow for launching placeholder-backed `slurm_to_openstack` executions from the dashboard.
