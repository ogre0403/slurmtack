## Why

Operators can already constrain placeholder jobs by Slurm constraint, but they cannot direct those jobs to a specific Slurm partition. That makes `slurm_to_openstack` requests hard to target in clusters where GPU nodes are split across multiple partitions or where the default partition is not appropriate.

## What Changes

- Add an optional `slurm_partition` field to the switch request for `slurm_to_openstack` executions
- Persist the requested partition with the execution so it survives async orchestration and restarts
- Pass the requested partition into placeholder job submission when the orchestrator or allocation handler asks Slurm for a node
- Preserve current behavior when no partition is provided, so Slurm continues using its default partition selection
- Add tests covering request binding, execution persistence, and placeholder submission with and without a partition

## Capabilities

### New Capabilities
- None

### Modified Capabilities
- `rest-api`: Allow `POST /v1/switches` to accept an optional `slurm_partition` for `slurm_to_openstack` requests
- `gpu-node-switch-allocation`: Carry a requested Slurm partition through execution state and honor it when submitting the placeholder allocation job

## Impact

- **Affected packages**: `internal/api`, `internal/service`, `internal/domain`, `internal/store`, `internal/orchestrator`, `internal/slurm`
- **Affected behavior**: placeholder job submission for `slurm_to_openstack` executions can now target a specific partition when requested
- **API surface**: `POST /v1/switches` request body gains optional `slurm_partition`
- **Persistence**: execution records need to store requested partition alongside requested constraint
- **Breaking**: None; the new field is optional