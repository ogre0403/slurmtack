## MODIFIED Requirements

### Requirement: Submit placeholder job

The client SHALL submit a placeholder job to slurmrestd via `POST /slurm/v0.0.40/job/submit` using the effective workload identity headers (`X-SLURM-USER-NAME`, `X-SLURM-USER-TOKEN`) for that execution. It MUST request exclusive node allocation, include the specified constraint, partition, and account when provided, set `current_working_directory` to `/home/<workload-user>`, set `standard_output` and `standard_error` to files under `/home/<workload-user>`, resolve the placeholder SIF image as `/home/<workload-user>/<SLURM_SIF_PATH>/<effective-placeholder-sif-file>`, export `SLURM_API_USER` and `SLURM_JWT_TOKEN` that match the effective workload identity, and return the allocated job ID on success. The effective placeholder SIF filename MUST come from the execution-scoped persisted filename, with a daemon-default fallback only for migrated executions that predate the new persisted field.

#### Scenario: Successful job submission with account and default SIF filename

- **WHEN** SubmitPlaceholderJob is called with a valid execution ID, partition `gpu-maint`, account `proj-123`, workload user `alice`, daemon `SLURM_SIF_PATH=slurmtack/build/output`, and effective placeholder SIF filename `placeholder-agent.sif`
- **THEN** client sends POST to slurmrestd with the v0.0.40 job submit path, workload identity headers, and an exclusive job body whose `job.account` is `proj-123`
- **AND** the job body sets `current_working_directory` to `/home/alice` and writes stdout and stderr under `/home/alice/`
- **AND** the script references `singularity run /home/alice/slurmtack/build/output/placeholder-agent.sif`
- **AND** the script exports `SLURM_API_USER=alice` and the matching `SLURM_JWT_TOKEN`
- **AND** the client returns PlaceholderJobResult with the assigned JobID

#### Scenario: Successful job submission with request-time SIF filename override

- **WHEN** SubmitPlaceholderJob is called with a valid execution ID, workload user `alice`, daemon `SLURM_SIF_PATH=slurmtack/build/output`, and effective placeholder SIF filename `placeholder-agent-debug.sif`
- **THEN** the generated script references `singularity run /home/alice/slurmtack/build/output/placeholder-agent-debug.sif`
- **AND** the client returns PlaceholderJobResult with the assigned JobID

#### Scenario: Successful job submission without explicit account

- **WHEN** SubmitPlaceholderJob is called with a valid execution ID and no account
- **THEN** client sends POST to slurmrestd without an `account` field in the job body
- **AND** the job still resolves the SIF path under the workload user's home directory

#### Scenario: Submission rejected by Slurm

- **WHEN** slurmrestd returns an error (for example invalid partition, invalid account, or an unreadable placeholder SIF command path)
- **THEN** client returns a SlurmAPIError containing the HTTP status and slurmrestd error messages

### Requirement: Query placeholder job state

The client SHALL query placeholder job state from slurmrestd using the documented job-state endpoint relative to the configured `SLURM_API_URL`, passing the effective workload identity headers (`X-SLURM-USER-NAME`, `X-SLURM-USER-TOKEN`) for that execution. The client MUST expose the raw Slurm state string together with normalized terminal-state semantics so the daemon can distinguish a still-waiting placeholder job from a terminal pre-allocation outcome.

#### Scenario: Query running placeholder job state

- **WHEN** the daemon queries placeholder job state for job `12345`
- **AND** slurmrestd reports a non-terminal state such as `PENDING` or `RUNNING`
- **THEN** the client returns that raw state
- **AND** it marks the job as non-terminal

#### Scenario: Query failed placeholder job state

- **WHEN** the daemon queries placeholder job state for job `12345`
- **AND** slurmrestd reports a terminal state such as `FAILED`
- **THEN** the client returns that raw state
- **AND** it marks the job as terminal so the daemon can stop the allocation wait

#### Scenario: Job-state query is rejected by Slurm

- **WHEN** slurmrestd rejects the job-state query for the supplied workload identity or job ID
- **THEN** the client returns a SlurmAPIError containing the HTTP status and slurmrestd error messages
