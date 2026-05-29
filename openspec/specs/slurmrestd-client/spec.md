## ADDED Requirements

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

### Requirement: Connection and timeout handling

The client SHALL use a configured HTTP timeout (default 30 seconds) for all requests. Connection failures MUST be distinguishable from API-level errors.

#### Scenario: Request timeout

- **WHEN** slurmrestd does not respond within the configured timeout
- **THEN** client returns a timeout error (context.DeadlineExceeded or equivalent)

#### Scenario: Connection refused

- **WHEN** slurmrestd is unreachable (connection refused)
- **THEN** client returns a connection error distinct from a slurmrestd API error

### Requirement: Structured error type

The client SHALL define a `SlurmAPIError` type that includes the HTTP status code and the list of error messages from slurmrestd's `errors` array. This type MUST implement the `error` interface.

#### Scenario: Error inspection

- **WHEN** a slurmrestd call fails with a parseable error response
- **THEN** the returned error can be type-asserted to SlurmAPIError to inspect StatusCode and Messages

### Requirement: Structured slurmrestd request logs

The client SHALL emit one structured `log/slog` entry for every outbound slurmrestd HTTP request after the request finishes or fails. Each log entry MUST include the HTTP method, request path, identity type used for the request (`workload` or `admin`), and elapsed time. When a response is received, the log entry MUST include the HTTP status code. The log entry MUST NOT include raw Slurm token values or full request bodies.

#### Scenario: Successful workload request is logged

- **WHEN** `GetNodeState` or `CancelJob` completes successfully
- **THEN** the client emits a structured log entry with the request method, request path, `identity` set to `workload`, the response status code, and latency

#### Scenario: Successful admin mutation is logged

- **WHEN** `DrainNode` or `ResumeNode` completes successfully using configured admin credentials
- **THEN** the client emits a structured log entry with `identity` set to `admin`, the request method, the request path, the response status code, and latency

#### Scenario: API rejection is logged with safe error summary

- **WHEN** slurmrestd returns an HTTP error response that is decoded into a `SlurmAPIError`
- **THEN** the client emits a structured log entry that includes the request method, request path, response status code, latency, and a summarized API error field
- **AND** the log does not include raw token header values

#### Scenario: Transport failure is logged

- **WHEN** the HTTP request fails before any response is received because of timeout, connection refusal, or context cancellation
- **THEN** the client emits a structured log entry with the request method, request path, `identity`, latency, and transport error details
