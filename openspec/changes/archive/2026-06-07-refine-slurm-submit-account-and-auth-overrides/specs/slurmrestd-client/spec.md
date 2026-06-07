## MODIFIED Requirements

### Requirement: Submit placeholder job

The client SHALL submit a placeholder job to slurmrestd via `POST /slurm/v0.0.40/job/submit` using the effective workload identity headers (`X-SLURM-USER-NAME`, `X-SLURM-USER-TOKEN`) for that execution. It MUST request exclusive node allocation, include the specified constraint, partition, and account when provided, set `current_working_directory` to `/home/<workload-user>`, set `standard_output` and `standard_error` to files under `/home/<workload-user>`, export `SLURM_API_USER` and `SLURM_JWT_TOKEN` that match the effective workload identity, and return the allocated job ID on success.

#### Scenario: Successful job submission with account and home-directory I/O paths

- **WHEN** SubmitPlaceholderJob is called with a valid execution ID, partition `gpu-maint`, account `proj-123`, and workload user `alice`
- **THEN** client sends POST to slurmrestd with the v0.0.40 job submit path, workload identity headers, and an exclusive job body whose `job.account` is `proj-123`
- **AND** the job body sets `current_working_directory` to `/home/alice` and writes stdout and stderr under `/home/alice/`
- **AND** the script exports `SLURM_API_USER=alice` and the matching `SLURM_JWT_TOKEN`
- **AND** the client returns PlaceholderJobResult with the assigned JobID

#### Scenario: Successful job submission without explicit account

- **WHEN** SubmitPlaceholderJob is called with a valid execution ID and no account
- **THEN** client sends POST to slurmrestd without an `account` field in the job body
- **AND** the job still uses the workload user's home directory for working, stdout, and stderr paths

#### Scenario: Submission rejected by Slurm

- **WHEN** slurmrestd returns an error (for example invalid partition, invalid account, or unresolvable constraint)
- **THEN** client returns a SlurmAPIError containing the HTTP status and slurmrestd error messages

### Requirement: JWT authentication

The client SHALL include the resolved Slurm identity headers on every request to slurmrestd. SubmitPlaceholderJob, GetNodeState, and CancelJob MUST use the execution's effective workload identity when one is stored for that execution; otherwise they MUST use the configured daemon workload identity. DrainNode and ResumeNode MUST use the admin identity when configured, and MUST fall back to the workload identity when admin credentials are not configured.

#### Scenario: Authenticated workload request uses execution override

- **WHEN** SubmitPlaceholderJob, GetNodeState, or CancelJob is called for an execution with request-scoped workload credentials
- **THEN** the HTTP request includes `X-SLURM-USER-NAME: <override-user>` and `X-SLURM-USER-TOKEN: <override-token>`

#### Scenario: Authenticated workload request uses configured fallback

- **WHEN** SubmitPlaceholderJob, GetNodeState, or CancelJob is called for an execution without request-scoped overrides and the daemon has a configured workload identity
- **THEN** the HTTP request includes `X-SLURM-USER-NAME: <workload-user>` and `X-SLURM-USER-TOKEN: <workload-token>`

#### Scenario: Authenticated admin mutation request

- **WHEN** DrainNode or ResumeNode is called and admin credentials are configured
- **THEN** the HTTP request includes `X-SLURM-USER-NAME: <admin-user>` and `X-SLURM-USER-TOKEN: <admin-token>`

#### Scenario: Token rejected

- **WHEN** slurmrestd returns HTTP 401
- **THEN** client returns an error indicating authentication failure
