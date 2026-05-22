## ADDED Requirements

### Requirement: Submit placeholder job

The client SHALL submit a placeholder job to slurmrestd via `POST /slurm/v0.0.38/job/submit` with exclusive node allocation, the specified constraint, and the specified partition. It MUST return the allocated job ID on success.

#### Scenario: Successful job submission

- **WHEN** SubmitPlaceholderJob is called with a valid execution ID, constraint, and partition
- **THEN** client sends POST to slurmrestd with exclusive job body and returns PlaceholderJobResult with the assigned JobID

#### Scenario: Submission rejected by Slurm

- **WHEN** slurmrestd returns an error (e.g., invalid partition, constraint unresolvable)
- **THEN** client returns a SlurmAPIError containing the HTTP status and slurmrestd error messages

### Requirement: Get node state

The client SHALL query node information via `GET /slurm/v0.0.38/node/{name}` and return the node's state, GRES list, and running jobs.

#### Scenario: Query existing node

- **WHEN** GetNodeState is called with a valid node name
- **THEN** client returns NodeState with NodeName, State (e.g., "idle", "drained", "mixed"), GRES list, and RunningJob list

#### Scenario: Query non-existent node

- **WHEN** GetNodeState is called with a node name that does not exist in Slurm
- **THEN** client returns an error indicating the node was not found

### Requirement: Drain node

The client SHALL drain a node via `POST /slurm/v0.0.38/node/{name}` with state set to "drain" and the provided reason string.

#### Scenario: Successful drain

- **WHEN** DrainNode is called with a valid node name and reason
- **THEN** client sends POST with drain state and reason, and returns nil on success

#### Scenario: Node already draining

- **WHEN** DrainNode is called on a node already in drain/drained state
- **THEN** client returns nil (idempotent — not an error)

#### Scenario: Drain fails

- **WHEN** slurmrestd returns an error for the drain request
- **THEN** client returns a SlurmAPIError with the error details

### Requirement: Resume node

The client SHALL resume a node via `POST /slurm/v0.0.38/node/{name}` with state set to "resume".

#### Scenario: Successful resume

- **WHEN** ResumeNode is called with a valid node name
- **THEN** client sends POST with resume state and returns nil on success

#### Scenario: Resume fails

- **WHEN** slurmrestd returns an error for the resume request
- **THEN** client returns a SlurmAPIError with the error details

### Requirement: Cancel job

The client SHALL cancel a job via `DELETE /slurm/v0.0.38/job/{id}`.

#### Scenario: Successful cancellation

- **WHEN** CancelJob is called with a valid job ID
- **THEN** client sends DELETE and returns nil on success

#### Scenario: Job not found

- **WHEN** CancelJob is called with a non-existent job ID
- **THEN** client returns an error indicating the job was not found

### Requirement: JWT authentication

The client SHALL include the configured JWT token as a Bearer token in the Authorization header for every request to slurmrestd.

#### Scenario: Authenticated request

- **WHEN** any method is called
- **THEN** the HTTP request includes header `Authorization: Bearer <token>`

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
