## Requirements

### Requirement: Display partition-scoped node inventory

The system SHALL provide an operator dashboard at `/` that loads a same-origin inventory read model and presents nodes grouped by Slurm partition. The primary inventory view MUST only include nodes discovered from Slurm partition membership, and the dashboard MUST allow the operator to scope the visible grid to a selected partition or to all discovered partitions.

#### Scenario: Load dashboard inventory for all partitions

- **WHEN** an operator opens the dashboard and the same-origin inventory request succeeds
- **THEN** the page shows the discovered Slurm partitions and a node inventory derived from those partitions

#### Scenario: Filter dashboard by selected partition

- **WHEN** the operator selects a specific partition such as `gpu-maint`
- **THEN** the dashboard shows only nodes that belong to that partition while preserving each node's canonical status fields

### Requirement: Show ownership and readiness summary per node

The dashboard SHALL display a status card or row for each discovered node that includes `node_name`, `partitions`, a normalized owner classification, a Slurm summary, an OpenStack summary, any active execution summary, and the most recent completed or failed execution summary. The owner classification MUST support at least `slurm`, `openstack`, `switching`, `conflict`, and `unknown`.

#### Scenario: Node is actively switching

- **WHEN** the inventory response marks a node with an active execution
- **THEN** the dashboard shows that node as `switching` and includes the active execution identifier and current state

#### Scenario: Node ownership is ambiguous

- **WHEN** the inventory response classifies a node as `conflict` or `unknown`
- **THEN** the dashboard renders that classification distinctly instead of implying ownership by either Slurm or OpenStack

### Requirement: Configure Slurm job settings for dashboard-triggered placeholder submission
The dashboard SHALL expose a Slurm job settings control in the top-right header area. That settings UI MUST let the operator edit `slurm_user_token`, `slurm_account`, and `placeholder_sif_file`; MUST derive one effective Slurm workload username from the JWT payload using a deterministic supported-claim precedence; MUST show the derived username as read-only feedback instead of asking the operator to type it; MUST fetch safe dashboard metadata for the configured home-relative `SLURM_SIF_PATH`; and MUST render a read-only expected SIF location hint as `/home/<derived-workload-user>/<SLURM_SIF_PATH>/<placeholder_sif_file>` whenever those inputs are all available. To prevent credential leakage, the sensitive `slurm_user_token` MUST be stored in `sessionStorage`, whereas `slurm_account` and `placeholder_sif_file` MUST be stored in `localStorage`. The dashboard MUST treat `slurm_user_token`, the derived workload username, `slurm_account`, and `placeholder_sif_file` as the complete required settings profile for dashboard-triggered `slurm_to_openstack`. If any part of that profile is missing, malformed, or undecodable, the dashboard MUST show a validation error and MUST NOT allow the `slurm_to_openstack` action to proceed. If the expected SIF location cannot be resolved because the token-derived user is unavailable, the SIF filename is blank, or the daemon has no usable `SLURM_SIF_PATH` metadata, the dashboard MUST show operator-visible guidance describing which input or config is missing.

#### Scenario: Save settings and show derived workload user with expected SIF location
- **WHEN** the operator opens the header settings UI and enters a Slurm JWT whose payload resolves to workload user `alice`, plus `slurm_account=proj-123` and `placeholder_sif_file=placeholder-agent-debug.sif`
- **AND** the dashboard metadata reports `SLURM_SIF_PATH=slurmtack/build/output`
- **THEN** the dashboard stores `slurm_user_token` in `sessionStorage` and other values in `localStorage`
- **AND** the settings UI shows `alice` as the derived read-only workload user
- **AND** the settings UI shows `/home/alice/slurmtack/build/output/placeholder-agent-debug.sif` as the expected SIF location

#### Scenario: Restore settings after page reload with expected SIF location
- **WHEN** the operator previously saved Slurm job settings in the dashboard
- **AND** the dashboard page is reloaded (F5)
- **AND** the dashboard metadata reports `SLURM_SIF_PATH=slurmtack/build/output`
- **THEN** the settings UI restores the saved token from `sessionStorage` and other settings from `localStorage`
- **AND** the dashboard recomputes and displays the derived workload user from the restored token
- **AND** the dashboard recomputes and displays `/home/<derived-workload-user>/slurmtack/build/output/<placeholder_sif_file>` as the expected SIF location

#### Scenario: Reject unusable Slurm token in settings
- **WHEN** the operator enters a malformed JWT or a JWT that does not contain any supported username claim
- **THEN** the dashboard shows an operator-visible validation error for the Slurm job settings
- **AND** the dashboard does not use that token for later `slurm_to_openstack` submission
- **AND** the expected SIF location hint explains that a valid token-derived workload user is required before the home path can be resolved

#### Scenario: Explain unresolved SIF location when daemon path metadata is unavailable
- **WHEN** the operator has entered a valid Slurm token and `placeholder_sif_file=placeholder-agent-debug.sif`
- **AND** the dashboard metadata indicates that `SLURM_SIF_PATH` is not configured
- **THEN** the settings UI does not fabricate an absolute SIF path
- **AND** it tells the operator that the daemon `SLURM_SIF_PATH` configuration is required to determine the expected SIF location

#### Scenario: Reject incomplete required Slurm job settings
- **WHEN** the operator leaves `slurm_account` or `placeholder_sif_file` empty, or no valid workload username can be derived from the configured token
- **THEN** the dashboard marks the Slurm job settings as incomplete for `slurm_to_openstack`
- **AND** the operator cannot execute the dashboard `slurm_to_openstack` action until the settings are completed

### Requirement: Hybrid Client Storage
The dashboard SHALL use browser storage to persist operator settings. To safeguard high-value credentials, the dashboard MUST store sensitive credentials (`slurm_user_token` and `slurmtack_token`) exclusively in `sessionStorage` (which persists across page refreshes but is cleared on tab close). The dashboard MUST store non-sensitive settings (`slurm_account` and `placeholder_sif_file`) in `localStorage` to ensure they survive across browser sessions.

#### Scenario: Verify storage location of sensitive and non-sensitive fields
- **WHEN** the operator saves the Slurm job settings in the UI
- **THEN** the sensitive `slurm_user_token` is written to `sessionStorage` and NOT to `localStorage`
- **AND** the non-sensitive `slurm_account` and `placeholder_sif_file` are written to `localStorage`

#### Scenario: Clearing settings wipes all storage
- **WHEN** the operator clicks the "Clear" button in the settings UI
- **THEN** both `sessionStorage` and `localStorage` keys are cleared of the respective settings

### Requirement: Silent Token Auto-Renewal
The dashboard SHALL handle `401 Unauthorized` API responses from the slurmtack server by executing a background silent renewal. If `slurm_user_token` exists in `sessionStorage`, the dashboard MUST pause the pending request, submit a background `POST /v1/auth/login` exchange request, save the new `slurmtack_token` in `sessionStorage` on success, and transparently retry the original API request. If the background exchange fails (e.g., due to an expired Slurm token), the dashboard MUST clear the sessionStorage tokens, display an explicit authentication error banner, and display the settings panel to prompt the operator for a new Slurm token.

#### Scenario: Successful silent token renewal
- **WHEN** the dashboard sends an API request and receives a `401 Unauthorized`
- **AND** a valid `slurm_user_token` is present in `sessionStorage`
- **AND** the background exchange request `POST /v1/auth/login` returns HTTP 200 with a new token
- **THEN** the dashboard saves the new token in `sessionStorage`
- **AND** the dashboard retries and successfully completes the original API request with zero visible error to the operator

#### Scenario: Unsuccessful silent token renewal forces login prompt
- **WHEN** the dashboard sends an API request and receives a `401 Unauthorized`
- **AND** the background exchange request `POST /v1/auth/login` fails with HTTP 401
- **THEN** the dashboard clears the tokens from `sessionStorage`
- **AND** the dashboard displays an error banner: "Your Slurm Token has expired. Please re-enter it."
- **AND** the dashboard automatically opens the settings panel for token input

### Requirement: Trigger switch actions from the dashboard

The dashboard SHALL let an operator trigger `openstack_to_slurm` and `slurm_to_openstack` transitions by calling the existing switch creation API. For `openstack_to_slurm`, the dashboard MUST submit the selected `node_name` from the node card. For `slurm_to_openstack`, the dashboard MUST expose the action from partition-scoped controls rather than an individual node card because request-time node selection is unsupported. The dashboard MUST NOT send a `slurm_to_openstack` request unless the required Slurm job settings profile is complete and valid. When that profile is complete, the dashboard MUST include `slurm_account`, `placeholder_sif_file`, `slurm_user`, and `slurm_user_token` in the `slurm_to_openstack` request. When the current partition selection is `All`, the dashboard MUST send `POST /v1/switches` with `direction=slurm_to_openstack`, the complete Slurm job settings profile, no `node_name`, and no `slurm_partition`. When the current partition selection is a specific partition such as `gpu-maint`, the dashboard MUST send `POST /v1/switches` with `direction=slurm_to_openstack`, `slurm_partition=gpu-maint`, and the complete Slurm job settings profile without sending `node_name`. If the required Slurm job settings profile is incomplete or invalid, the dashboard MUST block the action before any switch request is sent and MUST show the operator why the action is unavailable. The dashboard MUST also expose cancellation for an active execution by calling the existing cancel endpoint.

#### Scenario: Launch openstack_to_slurm from a node card

- **WHEN** the operator starts an `openstack_to_slurm` action for node `gpu-01`
- **THEN** the dashboard sends `POST /v1/switches` with `direction=openstack_to_slurm` and `node_name=gpu-01`

#### Scenario: Block slurm_to_openstack when Slurm job settings are incomplete

- **WHEN** the operator selects any partition context and starts `slurm_to_openstack` from the partition-scoped action control
- **AND** the required Slurm job settings profile is incomplete or invalid
- **THEN** the dashboard does not send `POST /v1/switches`
- **AND** the dashboard shows the operator that Slurm token, derived user, account, and SIF filename must be configured before the switch can run

#### Scenario: Launch slurm_to_openstack from a specific partition with stored Slurm job settings

- **WHEN** the operator selects partition `gpu-maint` and starts `slurm_to_openstack` from the partition-scoped action control
- **AND** the stored Slurm job settings resolve to workload user `alice`, `slurm_account=proj-123`, and `placeholder_sif_file=placeholder-agent-debug.sif`
- **THEN** the dashboard sends `POST /v1/switches` with `direction=slurm_to_openstack`, `slurm_partition=gpu-maint`, `slurm_user=alice`, `slurm_user_token=<stored-token>`, `slurm_account=proj-123`, and `placeholder_sif_file=placeholder-agent-debug.sif`
- **AND** the request does not send `node_name`

#### Scenario: Launch slurm_to_openstack from All partition context with complete Slurm job settings

- **WHEN** the operator selects `All` in the partition panel and starts `slurm_to_openstack` from the partition-scoped action control
- **AND** the stored Slurm job settings resolve to workload user `alice`, `slurm_account=proj-123`, and `placeholder_sif_file=placeholder-agent-debug.sif`
- **THEN** the dashboard sends `POST /v1/switches` with `direction=slurm_to_openstack`, `slurm_user=alice`, `slurm_user_token=<stored-token>`, `slurm_account=proj-123`, and `placeholder_sif_file=placeholder-agent-debug.sif`
- **AND** the request does not send `node_name` or `slurm_partition`

#### Scenario: Slurm-owned node card does not imply request-time node targeting

- **WHEN** the dashboard renders a node card for a node whose available direction is `slurm_to_openstack`
- **THEN** the node card does not render a `slurm_to_openstack` action button
- **AND** the partition-scoped action control remains the place where the operator launches that workflow

#### Scenario: Cancel active execution from a node summary

- **WHEN** the operator clicks cancel for an execution currently shown as active on a node
- **THEN** the dashboard sends `POST /v1/switches/<id>/cancel`
- **AND** it reflects the accepted cancellation state

#### Scenario: Cancel active execution from the execution panel

- **WHEN** the execution-focused right-side panel renders an execution whose `overall_status` is `active`
- **AND** the operator clicks cancel for that execution
- **THEN** the dashboard sends `POST /v1/switches/<id>/cancel`
- **AND** it refreshes the execution row and selected detail view to reflect the accepted cancellation state

### Requirement: Browse execution history and inspect step timelines

The dashboard SHALL provide an execution-focused right-side panel with a paginated execution list and a drilldown panel for an individual execution. The execution list MUST include recent active executions and prior executions returned by the API, show 10 executions per page by default, and display each execution's direction, node binding when known, `current_state`, `overall_status`, and request time. Selecting an execution MUST show the execution summary and current state in the detail panel, and the drilldown panel MUST render the ordered step timeline returned for the selected execution with fine-grained metadata for each step, including step name, sequence, status, started time, ended time when present, host when present, retry count, exit code or failure classification when present, and any recorded output or snapshot paths as secondary detail. The dashboard MAY continue to offer node and outcome filters, but changing filters MUST refresh the execution list and reset pagination to the first page.

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
- **THEN** the dashboard shows that step's `error_class` or `exit_code` when present
- **AND** it also shows any recorded stdout, stderr, or snapshot path metadata when present

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

### Requirement: Bootstrap dashboard session authentication

The dashboard SHALL establish API authentication through the Slurm token exchange flow instead of any shared static API token. When a stored `slurm_user_token` resolves to a workload username and no valid `slurmtack_token` is currently available, the dashboard MUST call `POST /v1/auth/login`, store the returned `slurmtack_token` in `sessionStorage`, and use that token for subsequent protected API requests. The dashboard MUST NOT prompt for, persist, or send a legacy `API_TOKEN`.

#### Scenario: Stored Slurm token bootstraps a dashboard session
- **WHEN** the operator reloads the dashboard and `sessionStorage` contains a valid `slurm_user_token` but no `slurmtack_token`
- **THEN** the dashboard automatically exchanges the Slurm token for a new `slurmtack_token`
- **AND** the dashboard stores the returned session token in `sessionStorage` before loading protected API data

#### Scenario: Missing or invalid stored Slurm token does not fall back to API token prompt
- **WHEN** the dashboard loads without a usable `slurm_user_token` or the startup exchange request fails
- **THEN** the dashboard keeps the operator unauthenticated for protected API calls
- **AND** the dashboard opens the settings panel or shows an authentication error instead of prompting for a shared API token
