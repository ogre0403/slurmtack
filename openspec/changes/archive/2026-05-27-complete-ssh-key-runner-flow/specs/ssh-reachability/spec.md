## ADDED Requirements

### Requirement: Configured SSH runner transport

The daemon SHALL use a configured SSH runner transport for reboot and reachability operations. That transport MUST apply the configured `SSH_USER`, `SSH_PORT`, `SSH_OPTIONS`, and `SSH_PRIVATE_KEY_PATH` values to both the `reboot` command and the post-reboot `hostname` probe.

#### Scenario: Reboot command uses configured identity

- **WHEN** an execution reaches the reboot step and the daemon has `SSH_USER=slurm`, `SSH_PORT=2222`, `SSH_PRIVATE_KEY_PATH=/run/secrets/node-key`, and `SSH_OPTIONS=StrictHostKeyChecking=accept-new ConnectTimeout=5`
- **THEN** the daemon issues the `reboot` command through the SSH runner using target `slurm@<host>`, port `2222`, the configured private key file, and the configured SSH options

#### Scenario: Reachability probe uses the same transport

- **WHEN** an execution is in state `rebooting` and the orchestrator polls host reachability
- **THEN** each `hostname` probe uses the same configured SSH user, port, private key file, and SSH options as the reboot command