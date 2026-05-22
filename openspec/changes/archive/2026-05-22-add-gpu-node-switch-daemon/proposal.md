## Why

GPU nodes need to move between Slurm worker mode and OpenStack compute mode without requiring an operator to manually coordinate control-plane actions, host reconfiguration, and failure recovery. The current design work in [docs/switch-design.md](/workspaces/slurmtack/docs/switch-design.md) is detailed enough to turn into a concrete OpenSpec change, and the next step is to define the required behavior for a daemon that can execute these transitions safely and asynchronously.

## What Changes

- Add a daemon-driven GPU node switch workflow that accepts asynchronous requests and tracks each request as an execution with explicit state transitions.
- Add behavior for Slurm-to-OpenStack switching that uses a placeholder Slurm job to discover and isolate the allocated node before any node-bound mutation begins.
- Add behavior for OpenStack-to-Slurm switching that verifies the node is free of compute workloads before host reconfiguration and Slurm reattachment.
- Add requirements for per-node locking, idempotent event handling, retry-safe execution identifiers, and explicit failure classifications.
- Add requirements for structured execution records, step records, deterministic log layout, reboot diagnostics, and rollback or manual-recovery classification.

## Capabilities

### New Capabilities
- `gpu-node-switch-orchestration`: Accept, coordinate, and finalize asynchronous GPU node ownership transitions between Slurm and OpenStack.
- `gpu-node-switch-allocation`: Reserve and identify a Slurm-owned GPU node through a placeholder job before node-bound switching begins.
- `gpu-node-switch-observability`: Persist execution evidence, step timelines, and host or control-plane diagnostics for replayable debugging and recovery.

### Modified Capabilities

None.

## Impact

Affected systems include the future switch daemon, the persistent state store, the message bus, the SSH command wrapper used for host mutations, the Slurm control plane, and the OpenStack control plane. This change also defines the contract for execution logs and schemas that later implementation work in [cmd/main.go](/workspaces/slurmtack/cmd/main.go) and related packages will need to satisfy.