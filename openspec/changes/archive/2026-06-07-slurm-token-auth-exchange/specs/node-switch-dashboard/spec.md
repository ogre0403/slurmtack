## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: Configure Slurm job settings for dashboard-triggered placeholder submission
The dashboard SHALL expose a Slurm job settings control in the top-right header area. That settings UI MUST let the operator edit `slurm_user_token`, `slurm_account`, and `placeholder_sif_file`; MUST derive one effective Slurm workload username from the JWT payload using a deterministic supported-claim precedence; MUST show the derived username as read-only feedback instead of asking the operator to type it; and MUST persist the editable settings in browser storage so they survive page reloads. To prevent credential leakage, the sensitive `slurm_user_token` MUST be stored in `sessionStorage`, whereas `slurm_account` and `placeholder_sif_file` MUST be stored in `localStorage`. The dashboard MUST treat `slurm_user_token`, the derived workload username, `slurm_account`, and `placeholder_sif_file` as the complete required settings profile for dashboard-triggered `slurm_to_openstack`. If any part of that profile is missing, malformed, or undecodable, the dashboard MUST show a validation error and MUST NOT allow the `slurm_to_openstack` action to proceed.

#### Scenario: Save settings and show derived workload user
- **WHEN** the operator opens the header settings UI and enters a Slurm JWT whose payload resolves to workload user `alice`, plus `slurm_account=proj-123` and `placeholder_sif_file=placeholder-agent-debug.sif`
- **THEN** the dashboard stores `slurm_user_token` in `sessionStorage` and other values in `localStorage`
- **AND** the settings UI shows `alice` as the derived read-only workload user

#### Scenario: Restore settings after page reload
- **WHEN** the operator previously saved Slurm job settings in the dashboard
- **AND** the dashboard page is reloaded (F5)
- **THEN** the settings UI restores the saved token from `sessionStorage` and other settings from `localStorage`
- **AND** the dashboard recomputes and displays the derived workload user from the restored token
