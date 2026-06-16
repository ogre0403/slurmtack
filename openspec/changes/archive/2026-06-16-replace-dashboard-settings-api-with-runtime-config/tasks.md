## 1. Runtime Dashboard Config

- [x] 1.1 Add nginx runtime config generation that emits a writable `dashboard-config.js` from a safe whitelist including `SLURM_SIF_PATH`.
- [x] 1.2 Update the dashboard HTML and JavaScript to load `window.SLURMTACK_CONFIG` before `dashboard.js` runs and remove the `/v1/dashboard/settings` fetch path.
- [x] 1.3 Update container wiring so the nginx service receives the needed env input and serves the generated config asset with no-store caching.

## 2. API and Contract Cleanup

- [x] 2.1 Remove the dashboard settings handler, DTO, server option, route registration, and API tests tied to `GET /v1/dashboard/settings`.
- [x] 2.2 Regenerate or update Swagger artifacts and route annotations so `/v1/dashboard/settings` is no longer documented.

## 3. Verification and Documentation

- [x] 3.1 Update dashboard UI string tests to assert runtime-config-driven SIF hint behavior and absence of the old settings API call.
- [x] 3.2 Update operator-facing documentation to describe the runtime config source, restart expectations, and the reduced API surface.
