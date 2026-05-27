## MODIFIED Requirements

### Requirement: Configured SSH runner transport

The daemon SHALL use a configured SSH runner transport for reboot and reachability operations. That transport MUST apply the configured `SSH_USER`, `SSH_PORT`, `SSH_OPTIONS`, and `SSH_PRIVATE_KEY_PATH` values to both the `reboot` command and the post-reboot `hostname` probe. The transport MUST render the remote command payload from `Command` and `Args` only; `execution_id`, `step_name`, and other workflow metadata MAY be used for local correlation and logging but MUST NOT be appended to the remote command line.

#### Scenario: Reboot command uses configured identity

- **WHEN** an execution reaches the reboot step and the daemon has `SSH_USER=slurm`, `SSH_PORT=2222`, `SSH_PRIVATE_KEY_PATH=/run/secrets/node-key`, and `SSH_OPTIONS=StrictHostKeyChecking=accept-new ConnectTimeout=5`
- **THEN** the daemon issues the `reboot` command through the SSH runner using target `slurm@<host>`, port `2222`, the configured private key file, the configured SSH options, and a rendered remote command of exactly `reboot`

#### Scenario: Reachability probe uses the same transport

- **WHEN** an execution is in state `rebooting` and the orchestrator polls host reachability
- **THEN** each `hostname` probe uses the same configured SSH user, port, private key file, and SSH options as the reboot command, and the rendered remote command is exactly `hostname`

#### Scenario: Workflow metadata does not mutate the remote payload

- **WHEN** the daemon dispatches a reboot or reachability probe with `execution_id` and `step_name` metadata
- **THEN** that metadata is retained only for local correlation and does not appear in the remote command sent over SSH