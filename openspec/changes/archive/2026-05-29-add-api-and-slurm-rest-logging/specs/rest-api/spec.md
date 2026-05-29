## ADDED Requirements

### Requirement: Structured API access logs

The system SHALL emit one structured `log/slog` access log entry for every completed HTTP request handled by the daemon API, including unauthenticated `/health` requests and authenticated `/v1/` requests that succeed or fail authorization. Each access log entry MUST include the request method, the matched route pattern when available, the raw request path, the final HTTP status code, and request latency. The access log MUST NOT include bearer tokens or request bodies.

#### Scenario: Successful authenticated request is logged

- **WHEN** client sends an authenticated API request such as `GET /v1/switches/1234`
- **THEN** the daemon emits a structured access log entry for that request
- **AND** the entry includes `method`, route pattern `/v1/switches/:id`, raw path `/v1/switches/1234`, `status_code`, and latency fields

#### Scenario: Authorization failure is logged without token leakage

- **WHEN** client sends `POST /v1/switches` with a missing or invalid `Authorization` header
- **THEN** the daemon emits a structured access log entry with HTTP 401 for that request
- **AND** the log does not include the bearer token value or request body content

#### Scenario: Health endpoint is logged

- **WHEN** client sends `GET /health`
- **THEN** the daemon emits a structured access log entry for `/health` with the final HTTP status and latency
