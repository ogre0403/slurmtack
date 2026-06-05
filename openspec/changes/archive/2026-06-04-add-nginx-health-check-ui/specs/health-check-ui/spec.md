## ADDED Requirements

### Requirement: Serve a browser validation page

The system SHALL expose a static HTML validation page from the nginx container at `/`. The page MUST load without direct browser access to the slurmtack container and MUST include a visible health status area for the proxied backend check.

#### Scenario: Load validation page from nginx

- **WHEN** an operator opens the stack's external HTTP entrypoint at `/`
- **THEN** nginx returns the static HTML page with HTTP 200
- **AND** the page contains visible text or UI placeholders for the slurmtack health check result

### Requirement: Display proxied health status

The validation page SHALL request `GET /api/health` from the same origin and render the result returned through nginx. When the proxied request succeeds, the page MUST show that the backend is healthy. When the proxied request fails or returns an unhealthy response, the page MUST show a failure state without exposing internal container hostnames or direct slurmtack URLs.

#### Scenario: Health check succeeds through nginx proxy

- **WHEN** the page requests `GET /api/health`
- **AND** slurmtack responds to the proxied `GET /health` request with HTTP 200 and `{"status":"ok"}`
- **THEN** the page shows a healthy result based on the proxied response

#### Scenario: Health check fails through nginx proxy

- **WHEN** the page requests `GET /api/health`
- **AND** nginx cannot reach slurmtack or the backend returns a non-200 health response
- **THEN** the page shows that the backend is unavailable or unhealthy
- **AND** the page does not display the internal slurmtack container address
