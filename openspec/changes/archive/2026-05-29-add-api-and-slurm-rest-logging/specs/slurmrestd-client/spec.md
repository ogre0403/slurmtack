## ADDED Requirements

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
