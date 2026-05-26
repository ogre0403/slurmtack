## MODIFIED Requirements

### Requirement: Submit placeholder job

The client SHALL submit a placeholder job to slurmrestd via `POST /slurm/v0.0.40/job/submit` using the configured workload identity headers (`X-SLURM-USER-NAME`, `X-SLURM-USER-TOKEN`). It MUST request exclusive node allocation, include the specified constraint and partition when provided, and return the allocated job ID on success.

#### Scenario: Successful job submission

- **WHEN** SubmitPlaceholderJob is called with a valid execution ID, constraint, and partition
- **THEN** client sends POST to slurmrestd with the v0.0.40 job submit path, workload identity headers, and an exclusive job body, then returns PlaceholderJobResult with the assigned JobID

#### Scenario: Submission rejected by Slurm

- **WHEN** slurmrestd returns an error (e.g., invalid partition, constraint unresolvable)
- **THEN** client returns a SlurmAPIError containing the HTTP status and slurmrestd error messages

### Requirement: Get node state

The client SHALL query node information via `GET /slurm/v0.0.40/node/{name}` using the configured workload identity headers and return the node's state, GRES list, and running jobs.

#### Scenario: Query existing node

- **WHEN** GetNodeState is called with a valid node name
- **THEN** client sends a v0.0.40 node lookup request with the workload identity headers and returns NodeState with NodeName, State (for example `idle`, `drained`, `mixed`), GRES list, and RunningJob list

#### Scenario: Query non-existent node

- **WHEN** GetNodeState is called with a node name that does not exist in Slurm
- **THEN** client returns an error indicating the node was not found

### Requirement: Drain node

The client SHALL drain a node via `POST /slurm/v0.0.40/node/{name}` using the configured admin identity headers when they are configured, otherwise the workload identity headers. The request body MUST set `state` to `["DRAIN"]` and include the provided reason string.

#### Scenario: Successful drain

- **WHEN** DrainNode is called with a valid node name and reason
- **THEN** client sends POST with `{"state":["DRAIN"],"reason":"..."}` and returns nil on success

#### Scenario: Node already draining

- **WHEN** DrainNode is called on a node already in drain or drained state
- **THEN** client returns nil (idempotent and not an error)

#### Scenario: Drain fails

- **WHEN** slurmrestd returns an error for the drain request
- **THEN** client returns a SlurmAPIError with the error details

### Requirement: Resume node

The client SHALL resume a node via `POST /slurm/v0.0.40/node/{name}` using the configured admin identity headers when they are configured, otherwise the workload identity headers. The request body MUST set `state` to `["RESUME"]`.

#### Scenario: Successful resume

- **WHEN** ResumeNode is called with a valid node name
- **THEN** client sends POST with `{"state":["RESUME"]}` and returns nil on success

#### Scenario: Resume fails

- **WHEN** slurmrestd returns an error for the resume request
- **THEN** client returns a SlurmAPIError with the error details

### Requirement: Cancel job

The client SHALL cancel a job via `DELETE /slurm/v0.0.40/job/{id}` using the configured workload identity headers.

#### Scenario: Successful cancellation

- **WHEN** CancelJob is called with a valid job ID
- **THEN** client sends DELETE to the v0.0.40 job endpoint and returns nil on success

#### Scenario: Job not found

- **WHEN** CancelJob is called with a non-existent job ID
- **THEN** client returns an error indicating the job was not found

### Requirement: JWT authentication

The client SHALL include the configured Slurm identity headers on every request to slurmrestd. SubmitPlaceholderJob, GetNodeState, and CancelJob MUST use the workload identity. DrainNode and ResumeNode MUST use the admin identity when configured, and MUST fall back to the workload identity when admin credentials are not configured.

#### Scenario: Authenticated workload request

- **WHEN** SubmitPlaceholderJob, GetNodeState, or CancelJob is called
- **THEN** the HTTP request includes `X-SLURM-USER-NAME: <workload-user>` and `X-SLURM-USER-TOKEN: <workload-token>`

#### Scenario: Authenticated admin mutation request

- **WHEN** DrainNode or ResumeNode is called and admin credentials are configured
- **THEN** the HTTP request includes `X-SLURM-USER-NAME: <admin-user>` and `X-SLURM-USER-TOKEN: <admin-token>`

#### Scenario: Token rejected

- **WHEN** slurmrestd returns HTTP 401
- **THEN** client returns an error indicating authentication failure