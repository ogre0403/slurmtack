## ADDED Requirements

### Requirement: REST boundary logs use structured daemon logging and redaction rules

The system SHALL emit inbound API access logs and outbound slurmrestd request logs through the same structured daemon logging approach used elsewhere in the process. REST boundary log entries MUST identify their component and event names in a stable way, and they MUST follow the daemon's redaction rules by excluding bearer tokens, Slurm JWT tokens, and full HTTP body payloads.

#### Scenario: Operator correlates REST boundary logs with daemon logs

- **WHEN** an operator reviews daemon logs for an API-triggered switch workflow
- **THEN** the API access log and the related slurmrestd client logs appear as structured entries with stable component and event fields that can be filtered together with existing daemon trace logs

#### Scenario: Sensitive headers are not written to logs

- **WHEN** the daemon logs an inbound authenticated API request or an outbound slurmrestd request
- **THEN** the resulting structured log entry excludes bearer token values, Slurm token values, and raw HTTP body payloads
