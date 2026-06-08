## MODIFIED Requirements

### Requirement: Web Session Token Authentication

The system SHALL protect all `/v1/*` endpoints (except `/health` and `/v1/auth/login`) using the dynamic Web Session JWT. The authentication middleware MUST verify the signature, validity, and expiration of the JWT. If the token is valid, the middleware MUST extract the `sub` claim and bind it to the request context as the authenticated user. The middleware MUST reject legacy static API tokens and any other bearer token that is not a valid slurmtack-issued Web Session JWT.

#### Scenario: Authenticated request with valid Web Session JWT
- **WHEN** client sends a request with header `Authorization: Bearer <valid-web-jwt>`
- **THEN** the system authorizes the request and executes the handler with the authenticated user context bound

#### Scenario: Rejected request with expired Web Session JWT
- **WHEN** client sends a request with header `Authorization: Bearer <expired-web-jwt>`
- **THEN** the system rejects the request with HTTP 401 and body `{"error": "token expired"}`

#### Scenario: Rejected request with legacy static API token
- **WHEN** client sends a request with header `Authorization: Bearer <legacy-api-token>`
- **THEN** the system rejects the request with HTTP 401
- **AND** the request is not authorized through any static-token fallback
