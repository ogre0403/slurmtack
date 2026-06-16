## ADDED Requirements

### Requirement: Honor runtime-configured cloud partition scope in dashboard navigation and actions

When the dashboard runtime config exposes a non-empty `slurmCloudPartition`, the dashboard SHALL enter fixed-partition mode for that value. In fixed-partition mode the dashboard MUST request inventory for the configured partition, MUST render only that partition in the partition list, MUST NOT offer the "Show all partitions" option, and MUST send `slurm_partition=<slurmCloudPartition>` for dashboard-triggered `slurm_to_openstack` actions. The dashboard MUST limit switch controls to the nodes returned by that scoped inventory.

#### Scenario: Dashboard enters fixed-partition mode from runtime config

- **WHEN** the dashboard loads with runtime config reporting `slurmCloudPartition=gpu-cloud`
- **THEN** the partition panel shows only `gpu-cloud`
- **AND** it does not show a "Show all partitions" option
- **AND** the initial inventory load requests only `gpu-cloud`

#### Scenario: Dashboard sends the configured cloud partition for slurm_to_openstack

- **WHEN** the dashboard loads with runtime config reporting `slurmCloudPartition=gpu-cloud`
- **AND** the operator starts `slurm_to_openstack` from the partition-scoped action control
- **THEN** the dashboard sends `POST /v1/switches` with `direction=slurm_to_openstack` and `slurm_partition=gpu-cloud`
- **AND** it does not offer any alternative partition choice in the UI

## MODIFIED Requirements

### Requirement: Load safe dashboard runtime config for the SIF location hint

The dashboard SHALL load a safe runtime configuration object at page startup before executing `dashboard.js`. That runtime configuration MUST include `slurmSifPathConfigured` and `slurmSifPath` derived from deployment-time configuration, and it MAY include other explicitly whitelisted dashboard-safe values such as `slurmCloudPartition`. The dashboard MUST use the SIF-path fields when assembling the read-only expected SIF location hint, and it MUST use `slurmCloudPartition` when present to enter fixed-partition mode. The dashboard MUST NOT require `GET /v1/dashboard/settings` or any other dedicated API call to learn `SLURM_SIF_PATH` or the configured cloud partition.

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

#### Scenario: Dashboard reads cloud partition scope from runtime config

- **WHEN** the dashboard loads with runtime config reporting `slurmCloudPartition=gpu-cloud`
- **THEN** the dashboard applies `gpu-cloud` as its fixed partition scope
- **AND** it does so without calling any dedicated dashboard settings API

#### Scenario: Runtime config is limited to safe dashboard fields

- **WHEN** the nginx container generates the dashboard runtime configuration asset
- **THEN** the asset includes only explicitly whitelisted dashboard-safe fields such as `slurmSifPathConfigured`, `slurmSifPath`, and `slurmCloudPartition`
- **AND** it does not expose workload tokens, API credentials, or unrelated environment variables
