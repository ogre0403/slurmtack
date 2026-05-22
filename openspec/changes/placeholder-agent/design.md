## Context

The switch daemon submits a placeholder job to Slurm requesting one exclusive GPU node. Slurm's scheduler picks a node and runs the job there. The placeholder agent is the payload of that job — it runs inside a Singularity container on the allocated node and bridges information back to the daemon via RabbitMQ.

The agent's lifecycle is short and well-defined:
1. Start → discover hostname → publish allocation_event
2. Poll slurmrestd until node reaches drained state
3. Publish node_drained → exit 0

The daemon submits this job via slurmrestd using `sbatch --export=...` to pass configuration. The SIF image must be accessible from a shared filesystem path that all GPU nodes can read.

## Goals / Non-Goals

**Goals:**

- Self-contained binary that runs inside Singularity with no external dependencies beyond network access
- Publish exactly two MQ events with correct routing keys to the topology declared by Change 4
- Reliable drain polling with configurable interval
- Clean exit codes (0 = success, 1 = startup failure, 2 = poll timeout, 3 = MQ publish failure)
- Buildable as a static binary + SIF image

**Non-Goals:**

- Running outside Singularity (it can, but Singularity is the deployment target)
- Complex retry logic for MQ publish (one retry then exit with error code)
- Modifying node state (agent is read-only — daemon does the actual drain command)
- Multi-execution support (one agent per Slurm job per execution)
- Log shipping (stdout/stderr captured by Slurm's job output mechanism)

## Decisions

### Execution Model: Single-Shot Binary

**Choice**: The agent is a single `main()` that runs sequentially: discover → publish → poll → publish → exit.

**Alternatives considered**:
- Long-running daemon on the node: Overkill, lifecycle is bounded by the Slurm job
- Shell script + curl for MQ: Fragile, no structured error handling, harder to build into Singularity

**Rationale**: The agent has a clear linear lifecycle. No concurrency needed. A Go binary compiles to a single static executable that works perfectly in Singularity's minimal environment.

### Hostname Discovery

**Choice**: Use `os.Hostname()` to discover the allocated node name.

The agent runs exclusively on the allocated node (Slurm guarantees this via exclusive allocation). The hostname matches what Slurm reports as the node name.

### Environment Variables (passed via sbatch --export)

| Variable | Required | Description |
|----------|----------|-------------|
| `EXECUTION_ID` | yes | Binds this job to a specific daemon execution |
| `AMQP_URL` | yes | RabbitMQ connection string (amqp://...) |
| `SLURM_API_URL` | yes | slurmrestd base URL |
| `SLURM_JWT_TOKEN` | yes | JWT for slurmrestd access |
| `POLL_INTERVAL` | no | Drain poll interval (default: 5s) |
| `POLL_TIMEOUT` | no | Max time to wait for drain (default: 30m) |

### MQ Message Format

Matches the schema expected by the daemon's MQ consumer (Change 4):

```json
// routing key: execution.allocation
{
  "execution_id": "abc123",
  "job_id": "12345",
  "node_name": "gpu-01"
}

// routing key: execution.drained
{
  "execution_id": "abc123",
  "node_name": "gpu-01"
}
```

Published to exchange `gpu-switch.events` with the appropriate routing key.

### Drain Detection via slurmrestd

The agent polls `GET /slurm/v0.0.38/node/{hostname}` and checks the node state field. Drain is confirmed when state is one of: `drained`, `drained*`, `down`, `down*`.

Poll loop:
```
every POLL_INTERVAL:
  state = GET /slurm/v0.0.38/node/{hostname}
  if state in [drained, drained*, down, down*]:
    publish node_drained
    exit 0
  if elapsed > POLL_TIMEOUT:
    exit 2 (timeout)
```

### Singularity Definition File

```def
Bootstrap: library
From: alpine:3.19

%files
  placeholder-agent /usr/local/bin/placeholder-agent

%runscript
  exec /usr/local/bin/placeholder-agent

%labels
  Author slurmtack
  Description GPU switch placeholder agent
```

The binary is compiled as a static Linux/amd64 binary (`CGO_ENABLED=0`) and copied into the SIF at build time.

### sbatch Integration

The daemon's `SubmitPlaceholderJob` (slurm-client) will submit a job like:

```bash
sbatch \
  --job-name=gpu-switch-<execution_id> \
  --nodes=1 \
  --exclusive \
  --constraint=<slurm_constraint> \
  --partition=<partition> \
  --export=EXECUTION_ID=<id>,AMQP_URL=<url>,SLURM_API_URL=<url>,SLURM_JWT_TOKEN=<token> \
  --wrap="singularity run /shared/images/placeholder-agent.sif"
```

The SIF image path is configurable via daemon config (`PLACEHOLDER_SIF_PATH`).

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success — drain confirmed, both events published |
| 1 | Startup failure — missing env vars, cannot connect to MQ or slurmrestd |
| 2 | Poll timeout — node did not reach drained state within POLL_TIMEOUT |
| 3 | MQ publish failure — could not deliver event to RabbitMQ |

Slurm captures the exit code. The daemon can query job status to detect non-zero exits and classify failures.

## Risks / Trade-offs

- **Network access from Singularity** → The container needs to reach RabbitMQ and slurmrestd. Mitigation: Singularity uses host networking by default (no isolation), so this works naturally.
- **SIF image distribution** → The SIF must be on a shared filesystem visible to all GPU nodes. Mitigation: standard HPC pattern — shared FS is expected.
- **JWT token in environment** → Token is visible via `/proc/<pid>/environ` on the node. Mitigation: acceptable for staging; the token is the same one used by the daemon. Production could use a short-lived per-job token.
- **Hostname mismatch** → If `os.Hostname()` returns an FQDN but Slurm uses short names (or vice versa), the execution_id binding will fail. Mitigation: strip domain suffix if present, document naming convention.
- **Slurm kills the job** → If the job hits a wall-time limit or is cancelled externally, the agent dies without publishing node_drained. Mitigation: the daemon's orchestrator has timeouts for executions stuck in `source_quiescing` — it will fail the execution.
