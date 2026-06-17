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

### Requirement: Resolve admin-authenticated Slurm requests with renewable credentials

The client SHALL execute default node reads, partition listing, default job cancellation, drain, and resume through one effective admin identity. When `SSH_LOGIN_NODE` is unset, the effective admin identity MUST come from the configured static admin credentials with the existing fallback to the workload identity. When `SSH_LOGIN_NODE` is set, the client MUST obtain a short-lived admin token for `SLURM_ADMIN_USER` over the configured SSH login-node path before issuing an admin-authenticated request, MUST cache the current minted token in memory for reuse across later admin-authenticated requests, and MAY seed that cache from `SLURM_ADMIN_JWT_TOKEN` when one is configured.

Every successful admin token issuance performed over `SSH_LOGIN_NODE` MUST create one datastore audit record containing the issuance timestamp, the effective `SLURM_ADMIN_USER`, the configured `SSH_LOGIN_NODE`, and the renewal trigger. The renewal trigger MUST distinguish at least between an initial cache fill and a retry after admin-authentication failure. The datastore audit record MUST NOT contain the minted JWT value.

If an admin-authenticated request fails because the admin token is invalid or expired, the client MUST invalidate the cached admin token, mint a fresh token once, persist the renewal audit record, and retry the original admin-authenticated request once. If the retried request still fails for authentication reasons, the client MUST return that error. The client MUST NOT mint a new token or retry when the failure is unrelated to authentication.

#### Scenario: Static admin token is used when SSH renewal is disabled

- **WHEN** the client performs `DrainNode` or `ListPartitions` and `SSH_LOGIN_NODE` is unset
- **THEN** it sends the configured static admin identity headers to slurmrestd
- **AND** it does not attempt SSH-based token issuance

#### Scenario: First admin request mints a token over SSH

- **WHEN** the client performs an admin-authenticated Slurm request and `SSH_LOGIN_NODE=login-01` is set
- **AND** no cached admin token exists yet
- **THEN** the client obtains a short-lived token for `SLURM_ADMIN_USER` through the configured SSH login-node path
- **AND** it stores one datastore audit record for that issuance with trigger `cache_miss`
- **AND** it uses that minted token for the outgoing slurmrestd request

#### Scenario: Authentication failure refreshes the cached admin token once

- **WHEN** an admin-authenticated Slurm request receives an authentication failure from slurmrestd
- **AND** `SSH_LOGIN_NODE` is set
- **THEN** the client invalidates its cached admin token
- **AND** it mints one fresh admin token over SSH
- **AND** it stores one datastore audit record for that issuance with trigger `auth_failure`
- **AND** it retries the original admin-authenticated request once with the renewed token

#### Scenario: Repeated authentication failure returns an error

- **WHEN** an admin-authenticated Slurm request receives an authentication failure
- **AND** the single retry with a renewed token also receives an authentication failure
- **THEN** the client returns the Slurm error to the caller
- **AND** it does not continue retrying

#### Scenario: Non-authentication failure is not retried

- **WHEN** an admin-authenticated Slurm request fails due to an unrelated Slurm error such as invalid node state or a rejected operation
- **THEN** the client returns that error immediately
- **AND** it does not mint a new admin token for that failure
