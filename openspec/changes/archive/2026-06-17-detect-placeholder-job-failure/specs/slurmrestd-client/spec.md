## ADDED Requirements

### Requirement: Query placeholder job state

The client SHALL query placeholder job state from slurmrestd using the documented job-state endpoint relative to the configured `SLURM_API_URL`, passing the effective workload identity headers (`X-SLURM-USER-NAME`, `X-SLURM-USER-TOKEN`) for that execution. The client MUST expose the raw Slurm state string together with normalized terminal-state semantics so the daemon can distinguish a still-waiting placeholder job from a terminal pre-allocation outcome.

#### Scenario: Query running placeholder job state

- **WHEN** the daemon queries placeholder job state for job `12345`
- **AND** slurmrestd reports a non-terminal state such as `PENDING` or `RUNNING`
- **THEN** the client returns that raw state
- **AND** it marks the job as non-terminal

#### Scenario: Query failed placeholder job state

- **WHEN** the daemon queries placeholder job state for job `12345`
- **AND** slurmrestd reports a terminal state such as `FAILED`
- **THEN** the client returns that raw state
- **AND** it marks the job as terminal so the daemon can stop the allocation wait

#### Scenario: Job-state query is rejected by Slurm

- **WHEN** slurmrestd rejects the job-state query for the supplied workload identity or job ID
- **THEN** the client returns a SlurmAPIError containing the HTTP status and slurmrestd error messages
