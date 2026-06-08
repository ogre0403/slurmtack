## ADDED Requirements

### Requirement: Provide dashboard Slurm settings metadata
The system SHALL expose `GET /v1/dashboard/settings` for authenticated dashboard clients. The response MUST return only safe operator-guidance metadata needed by the dashboard settings UI and MUST NOT expose workload credentials or other secrets. For the SIF-location hint, the response MUST include `slurm_sif_path_configured` and `slurm_sif_path`, where `slurm_sif_path` is the configured home-relative `SLURM_SIF_PATH` value when present and an empty string otherwise. The system MUST return HTTP 200 whether or not `SLURM_SIF_PATH` is configured so the dashboard can distinguish "path unavailable" from transport failure.

#### Scenario: Return configured Slurm SIF path metadata
- **WHEN** an authenticated dashboard client sends `GET /v1/dashboard/settings`
- **AND** the daemon is configured with `SLURM_SIF_PATH=slurmtack/build/output`
- **THEN** the system returns HTTP 200 with a body that includes `{"slurm_sif_path_configured": true, "slurm_sif_path": "slurmtack/build/output"}`
- **AND** the response does not include workload JWTs, derived usernames, or expanded `/home/<user>` paths

#### Scenario: Return explicit unavailable state when Slurm SIF path is unset
- **WHEN** an authenticated dashboard client sends `GET /v1/dashboard/settings`
- **AND** the daemon does not have `SLURM_SIF_PATH` configured
- **THEN** the system returns HTTP 200 with a body that includes `{"slurm_sif_path_configured": false, "slurm_sif_path": ""}`
- **AND** the response lets the dashboard explain that the expected SIF location cannot yet be resolved
