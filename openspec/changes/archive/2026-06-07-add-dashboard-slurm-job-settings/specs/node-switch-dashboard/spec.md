## ADDED Requirements

### Requirement: Configure Slurm job settings for dashboard-triggered placeholder submission
The dashboard SHALL expose a Slurm job settings control in the top-right header area. That settings UI MUST let the operator edit `slurm_user_token`, `slurm_account`, and `placeholder_sif_file`; MUST derive one effective Slurm workload username from the JWT payload using a deterministic supported-claim precedence; MUST show the derived username as read-only feedback instead of asking the operator to type it; and MUST persist the editable settings in browser storage so they survive page reloads. The dashboard MUST treat `slurm_user_token`, the derived workload username, `slurm_account`, and `placeholder_sif_file` as the complete required settings profile for dashboard-triggered `slurm_to_openstack`. If any part of that profile is missing, malformed, or undecodable, the dashboard MUST show a validation error and MUST NOT allow the `slurm_to_openstack` action to proceed.

#### Scenario: Save settings and show derived workload user
- **WHEN** the operator opens the header settings UI and enters a Slurm JWT whose payload resolves to workload user `alice`, plus `slurm_account=proj-123` and `placeholder_sif_file=placeholder-agent-debug.sif`
- **THEN** the dashboard stores those editable values in browser storage
- **AND** the settings UI shows `alice` as the derived read-only workload user

#### Scenario: Restore settings after page reload
- **WHEN** the operator previously saved Slurm job settings in the dashboard
- **AND** the dashboard page is reloaded
- **THEN** the settings UI restores the saved token, account, and placeholder SIF filename from browser storage
- **AND** the dashboard recomputes and displays the derived workload user from the restored token

#### Scenario: Reject unusable Slurm token in settings
- **WHEN** the operator enters a malformed JWT or a JWT that does not contain any supported username claim
- **THEN** the dashboard shows an operator-visible validation error for the Slurm job settings
- **AND** the dashboard does not use that token for later `slurm_to_openstack` submission

#### Scenario: Reject incomplete required Slurm job settings
- **WHEN** the operator leaves `slurm_account` or `placeholder_sif_file` empty, or no valid workload username can be derived from the configured token
- **THEN** the dashboard marks the Slurm job settings as incomplete for `slurm_to_openstack`
- **AND** the operator cannot execute the dashboard `slurm_to_openstack` action until the settings are completed

## MODIFIED Requirements

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
