## ADDED Requirements

### Requirement: Guard duplicate OpenStack-to-Slurm request admission

Before creating an `openstack_to_slurm` execution, the system SHALL inspect the target node's current Slurm state and reject the request when that state already shows the node is back in active Slurm service. Active Slurm service MUST include schedulable states such as `idle`, `alloc`, or `mixed` when no `drain`, `drained`, or `down` token is present. When the request is rejected by this guard, the system MUST NOT persist an execution, publish MQ admission events, acquire a lease, or start any host mutation workflow.

#### Scenario: Active Slurm node is rejected before execution creation
- **WHEN** an operator submits `openstack_to_slurm` for node `gpu-01`
- **AND** Slurm reports `gpu-01` in `idle`
- **THEN** the system rejects the request as already under Slurm ownership
- **AND** no execution record is created

#### Scenario: Composite active state is rejected before execution creation
- **WHEN** an operator submits `openstack_to_slurm` for node `gpu-01`
- **AND** Slurm reports `gpu-01` in a schedulable state such as `mixed` with no drain/down token
- **THEN** the system rejects the request as already under Slurm ownership
- **AND** no MQ publish for that execution occurs

#### Scenario: Non-active Slurm node can still enter workflow
- **WHEN** an operator submits `openstack_to_slurm` for node `gpu-01`
- **AND** Slurm reports `gpu-01` in `drained`, `down`, or another resumable non-active state
- **THEN** the system accepts the request and continues the existing `openstack_to_slurm` workflow

#### Scenario: Slurm state cannot be determined at request time
- **WHEN** an operator submits `openstack_to_slurm` for node `gpu-01`
- **AND** the system cannot read the current Slurm node state
- **THEN** the request fails before execution creation
- **AND** the system does not start the switch workflow with incomplete ownership information
