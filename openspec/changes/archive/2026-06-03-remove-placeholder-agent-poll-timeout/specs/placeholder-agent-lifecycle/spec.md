## MODIFIED Requirements

### Requirement: Poll slurmrestd for drain state

The agent SHALL poll `GET /slurm/v0.0.40/node/{hostname}` at the configured interval (default 5s) to check if the node has entered the Slurm drain state required for the switch handoff. Each poll request MUST include `X-SLURM-USER-NAME` and `X-SLURM-USER-TOKEN` headers derived from the configured workload Slurm identity. Drain completion MUST be determined from Slurm state tokens rather than exact full-string matches. The agent MUST treat `drain`, `drained`, `drained*`, `down`, and `down*` tokens as satisfying the drain wait even when they appear inside a composite state such as `MIXED+DRAIN`. The agent MUST NOT enforce an internal overall poll timeout; the placeholder job runtime MUST instead be governed by Slurm job control such as partition time limits, cancellation, or successful drain completion.

#### Scenario: Node drains within polling window

- **WHEN** slurmrestd reports node state as `drained` during polling
- **THEN** agent proceeds to publish node_drained event

#### Scenario: Composite drain state completes polling

- **WHEN** slurmrestd reports node state as `MIXED+DRAIN`
- **THEN** agent considers the drain wait satisfied and proceeds to publish node_drained event

#### Scenario: Poll request uses Slurm identity headers

- **WHEN** agent issues a drain-state poll request
- **THEN** the HTTP request targets the v0.0.40 node endpoint and includes `X-SLURM-USER-NAME: <configured-or-default-user>` and `X-SLURM-USER-TOKEN: <workload-token>`

#### Scenario: slurmrestd unreachable during poll

- **WHEN** a single poll request fails (timeout or connection error)
- **THEN** agent logs warning and retries on next interval (does not exit immediately)

### Requirement: Exit codes

The agent SHALL use specific exit codes to communicate outcome to Slurm and the daemon: 0 (success), 1 (startup failure or local shutdown), and 3 (MQ publish failure). The agent MUST NOT emit a dedicated poll-timeout exit code because drain waiting is bounded by Slurm job policy rather than an internal deadline.

#### Scenario: Successful completion

- **WHEN** both events are published and drain is confirmed
- **THEN** agent exits with code 0

#### Scenario: Startup failure

- **WHEN** required env vars are missing, initial connections fail, or the process is cancelled before completion
- **THEN** agent exits with code 1

#### Scenario: MQ publish failure

- **WHEN** allocation or drained event publishing cannot be confirmed after the documented retry path
- **THEN** agent exits with code 3
