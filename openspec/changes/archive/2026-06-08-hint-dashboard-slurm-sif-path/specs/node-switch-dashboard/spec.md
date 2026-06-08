## MODIFIED Requirements

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
