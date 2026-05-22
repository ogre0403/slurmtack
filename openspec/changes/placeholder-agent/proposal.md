## Why

In the slurm-to-openstack flow, the daemon does not know which GPU node to switch until Slurm allocates one. The design requires a placeholder job that runs on the allocated node, reports which node it got (allocation_event), polls Slurm until the node is drained (node_drained event), then exits. This placeholder agent is the in-band signal bridge between Slurm's scheduler decisions and the switch daemon. Without it, the daemon cannot discover allocated nodes or confirm drain completion.

## What Changes

- Add a standalone Go binary (`cmd/placeholder-agent/`) that runs inside a Singularity container on a Slurm-allocated GPU node
- The agent discovers its hostname, reads execution context from environment variables, connects to RabbitMQ, and publishes two lifecycle events
- It polls slurmrestd to monitor node drain state until drain completes
- Add a Singularity definition file (`.def`) for building the container image
- Update the daemon's `SubmitPlaceholderJob` integration to submit the Singularity-wrapped agent instead of a dummy script

## Capabilities

### New Capabilities

- `placeholder-agent-lifecycle`: The agent binary's complete lifecycle — startup, allocation event publish, drain poll loop, drained event publish, and clean exit
- `placeholder-agent-packaging`: Singularity definition file and build process for packaging the agent as a container image deployable via Slurm sbatch

### Modified Capabilities

(none)

## Impact

- **New files**: `cmd/placeholder-agent/main.go`, `build/placeholder-agent.def`, build scripts
- **New dependencies**: `github.com/rabbitmq/amqp091-go` (already in go.mod from Change 4)
- **External systems**: RabbitMQ (publish), slurmrestd (poll node state)
- **Configuration**: Via environment variables passed through `sbatch --export`: `EXECUTION_ID`, `AMQP_URL`, `SLURM_API_URL`, `SLURM_JWT_TOKEN`, `POLL_INTERVAL`
- **Build artifacts**: Singularity SIF image containing the placeholder-agent binary
