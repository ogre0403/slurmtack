## MODIFIED Requirements

### Requirement: Browse execution history and inspect step timelines

The dashboard SHALL provide an execution-focused right-side panel with a paginated execution list and a drilldown panel for an individual execution. The execution list MUST include recent active executions and prior executions returned by the API, show 10 executions per page by default, and display each execution's direction, node binding when known, `current_state`, `overall_status`, and request time. Selecting an execution MUST show the execution summary and current state in the detail panel, and the drilldown panel MUST render the ordered step timeline returned for the selected execution with fine-grained metadata for each step, including step name, sequence, status, started time, ended time when present, host when present, retry count, exit code or failure classification when present, step `error_summary` when present, and any recorded output or snapshot paths as secondary detail. The dashboard MAY continue to offer node and outcome filters, but changing filters MUST refresh the execution list and reset pagination to the first page.

#### Scenario: Inspect execution details from the execution list

- **WHEN** the operator selects an execution from the right-side execution list
- **THEN** the dashboard shows the execution summary, its current state, and its ordered step records in the detail panel
- **AND** each rendered step shows at least its name, status, and timing metadata

#### Scenario: Running step remains visible while execution is in progress

- **WHEN** the selected execution includes a step whose `status` is `running`
- **AND** that step does not yet have `ended_at`
- **THEN** the dashboard renders that step as the current in-progress step instead of hiding it
- **AND** the rest of the persisted timeline remains visible around it

#### Scenario: Failed step surfaces operator-relevant failure detail

- **WHEN** the selected execution timeline includes a step with `status=failed`
- **THEN** the dashboard shows that step's `error_summary` when present
- **AND** it also shows `error_class` or `exit_code` when present
- **AND** it also shows any recorded stdout, stderr, or snapshot path metadata when present

#### Scenario: Blocked precheck shows readable refusal reason

- **WHEN** the selected execution timeline includes a failed precheck step classified as `precheck_blocked`
- **THEN** the execution detail view shows the operator-visible refusal reason from that step without requiring the operator to inspect external logs

#### Scenario: Browse the next page of executions

- **WHEN** the execution list contains more than 10 matching executions
- **AND** the operator navigates to the next page
- **THEN** the dashboard shows the next 10 executions from the list query
- **AND** it keeps the execution rows ordered consistently with the API response

#### Scenario: Browse the previous page of executions

- **WHEN** the operator has navigated beyond the first execution page
- **AND** the operator navigates back to the previous page
- **THEN** the dashboard restores the immediately preceding set of 10 execution rows

#### Scenario: Active execution remains actionable in the list

- **WHEN** the execution list includes an execution whose `overall_status` is `active`
- **THEN** the row shows the execution's current state
- **AND** the row includes a cancel control without requiring the operator to open the detail panel first

#### Scenario: Filter execution list by node or outcome

- **WHEN** the operator applies a filter such as `node=gpu-01` or `status=failed`
- **THEN** the dashboard refreshes the execution list using those filters
- **AND** it resets the paginated view to the first page of matching executions

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

### Requirement: Selected execution detail refreshes while polling

The dashboard SHALL refresh the currently selected execution detail view on the same polling cadence used for execution history while the detail drawer remains open. When the selected execution advances, fails, or completes between polls, the dashboard MUST refresh both the execution summary and the step timeline without requiring the operator to reselect that execution.

#### Scenario: Selected allocation wait updates to failed state

- **WHEN** the operator has selected an execution currently shown as `awaiting_source_allocation`
- **AND** a later poll finds that execution transitioned to `failed_non_destructive`
- **THEN** the dashboard updates the detail panel to show the failed current state and overall status
- **AND** it renders the failed `wait_for_source_allocation` step and its recorded failure summary without requiring the operator to reopen the panel
