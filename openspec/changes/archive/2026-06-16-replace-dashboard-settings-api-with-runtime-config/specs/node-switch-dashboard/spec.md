## ADDED Requirements

### Requirement: Load safe dashboard runtime config for the SIF location hint

The dashboard SHALL load a safe runtime configuration object at page startup before executing `dashboard.js`. That runtime configuration MUST include `slurmSifPathConfigured` and `slurmSifPath` derived from deployment-time configuration, and the dashboard MUST use those values when assembling the read-only expected SIF location hint. The dashboard MUST NOT require `GET /v1/dashboard/settings` or any other dedicated API call to learn `SLURM_SIF_PATH`.

#### Scenario: Dashboard computes expected SIF location from runtime config

- **WHEN** the dashboard loads with runtime config reporting `slurmSifPathConfigured=true` and `slurmSifPath=slurmtack/build/output`
- **AND** the operator enters a valid Slurm token that resolves to workload user `alice`
- **AND** the operator enters `placeholder-agent-debug.sif` as the placeholder SIF filename
- **THEN** the dashboard shows `/home/alice/slurmtack/build/output/placeholder-agent-debug.sif` as the expected SIF location
- **AND** it does so without calling a dedicated dashboard settings API

#### Scenario: Dashboard shows missing-config guidance from runtime config

- **WHEN** the dashboard loads with runtime config reporting `slurmSifPathConfigured=false`
- **AND** the operator has already provided a valid token-derived workload user
- **THEN** the dashboard keeps the SIF location unresolved
- **AND** it tells the operator that the daemon `SLURM_SIF_PATH` configuration is required to determine the expected SIF location

#### Scenario: Runtime config is limited to safe dashboard fields

- **WHEN** the nginx container generates the dashboard runtime configuration asset
- **THEN** the asset includes only explicitly whitelisted dashboard-safe fields such as `slurmSifPathConfigured` and `slurmSifPath`
- **AND** it does not expose workload tokens, API credentials, or unrelated environment variables
