## ADDED Requirements

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
