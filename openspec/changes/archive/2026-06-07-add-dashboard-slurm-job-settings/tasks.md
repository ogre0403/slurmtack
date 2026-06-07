## 1. Dashboard settings shell

- [x] 1.1 Update `docker/nginx/html/index.html` to add a top-right Slurm job settings entry point and the required UI surface for token, account, SIF filename, derived username, and validation feedback.
- [x] 1.2 Update `docker/nginx/html/dashboard.js` state/bootstrap logic to load, save, and clear the Slurm job settings profile from browser `localStorage`.

## 2. Slurm switch gating and request composition

- [x] 2.1 Implement dashboard-side JWT payload decoding with the documented username-claim precedence and show the derived workload user as read-only feedback.
- [x] 2.2 Refactor the partition-scoped `slurm_to_openstack` action so it blocks submission when the Slurm token, derived user, account, or SIF filename is incomplete or invalid and shows an operator-visible reason.
- [x] 2.3 When the Slurm job settings profile is complete, include `slurm_account`, `placeholder_sif_file`, `slurm_user`, `slurm_user_token`, and the optional selected `slurm_partition` in the `POST /v1/switches` payload while leaving `openstack_to_slurm` unchanged.

## 3. Verification and documentation

- [x] 3.1 Extend `internal/api/dashboard_ui_test.go` to assert the new settings UI region, settings persistence hooks, incomplete-settings blocking behavior, and enriched `slurm_to_openstack` payload fields.
- [x] 3.2 Update `docs/dashboard.md` to document the required Slurm job settings, browser-storage behavior, JWT-derived username display, and the fact that incomplete settings prevent dashboard-triggered `slurm_to_openstack`.
