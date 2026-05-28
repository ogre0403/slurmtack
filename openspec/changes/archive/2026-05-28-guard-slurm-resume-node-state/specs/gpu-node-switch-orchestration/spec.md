## ADDED Requirements

### Requirement: Guarded Slurm target attachment

For `openstack_to_slurm` executions, the system SHALL inspect the current Slurm node state before issuing a target-side `RESUME`. It MUST evaluate composite Slurm state strings token-by-token. If the node state includes `drain`, `drained`, or `down`, the system MUST issue `RESUME`. If the node is already schedulable in `idle`, `alloc`, or `mixed` and no drain/down token is present, the system MUST skip `RESUME` and continue the workflow. If the node is in any other state, the system MUST fail the attach step without issuing `RESUME`.

#### Scenario: Composite drain state resumes the node
- **WHEN** an `openstack_to_slurm` execution reaches target attachment and Slurm reports a state such as `drained+drain`, `idle+drain`, or `down`
- **THEN** the system issues `ResumeNode` for that node before continuing the workflow

#### Scenario: Already schedulable state skips resume
- **WHEN** an `openstack_to_slurm` execution reaches target attachment and Slurm reports `idle`, `alloc`, or `mixed` with no drain/down token
- **THEN** the system does not call `ResumeNode` and continues to verification using the current active node state

#### Scenario: Unsupported node state fails before mutation
- **WHEN** an `openstack_to_slurm` execution reaches target attachment and Slurm reports a node state that is neither resumable nor already schedulable
- **THEN** the system fails the attach step with an error describing the observed node state and does not call `ResumeNode`