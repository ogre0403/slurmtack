## 1. Types and Error Handling

- [ ] 1.1 Define `SlurmAPIError` struct in `internal/slurm/errors.go` with StatusCode, Messages fields, implementing `error` interface
- [ ] 1.2 Define slurmrestd response structs (job submit response, node info response, error response) in `internal/slurm/api_types.go`

## 2. Client Implementation

- [ ] 2.1 Create `internal/slurm/restclient.go` with constructor `NewRestClient(baseURL, jwtToken string, opts ...Option)` returning a `Client` interface implementation
- [ ] 2.2 Implement SubmitPlaceholderJob: POST `/slurm/v0.0.38/job/submit` with exclusive job body, parse job_id from response
- [ ] 2.3 Implement GetNodeState: GET `/slurm/v0.0.38/node/{name}`, parse NodeState fields (state, gres, running jobs)
- [ ] 2.4 Implement DrainNode: POST `/slurm/v0.0.38/node/{name}` with `{"state": "drain", "reason": "..."}`, handle idempotent case
- [ ] 2.5 Implement ResumeNode: POST `/slurm/v0.0.38/node/{name}` with `{"state": "resume"}`
- [ ] 2.6 Implement CancelJob: DELETE `/slurm/v0.0.38/job/{id}`
- [ ] 2.7 Add shared request helper: set Authorization Bearer header, handle timeout, parse error response body into SlurmAPIError

## 3. Unit Tests

- [ ] 3.1 Write unit tests with httptest.Server mocking slurmrestd responses for each method (success and error cases)
- [ ] 3.2 Test JWT header is present on all requests
- [ ] 3.3 Test timeout and connection error handling (distinguish from API errors)

## 4. Integration Tests

- [ ] 4.1 Create `internal/slurm/restclient_integration_test.go` gated with `//go:build integration`
- [ ] 4.2 Integration test: SubmitPlaceholderJob on test partition, verify job_id returned, then CancelJob to clean up
- [ ] 4.3 Integration test: GetNodeState for a known test node, verify fields populated
- [ ] 4.4 Integration test: DrainNode then ResumeNode on test node, verify state transitions
