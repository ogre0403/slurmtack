## 1. Agent Binary Core

- [x] 1.1 Create `cmd/placeholder-agent/main.go` with entrypoint: parse env vars, validate required vars, exit 1 on missing
- [x] 1.2 Implement hostname discovery via `os.Hostname()` with domain suffix stripping
- [x] 1.3 Implement structured JSON logger to stdout (timestamp, level, execution_id, message)
- [x] 1.4 Read `SLURM_JOB_ID` from environment (set by Slurm automatically)

## 2. MQ Publishing

- [x] 2.1 Implement AMQP connection (connect to AMQP_URL, open channel, confirm mode)
- [x] 2.2 Implement `publishAllocationEvent`: publish to `gpu-switch.events` with routing key `execution.allocation`, body: execution_id, job_id, node_name
- [x] 2.3 Implement `publishNodeDrainedEvent`: publish to `gpu-switch.events` with routing key `execution.drained`, body: execution_id, node_name
- [x] 2.4 Handle publish failures: one reconnect attempt, then exit code 3

## 3. Drain Poll Loop

- [x] 3.1 Implement slurmrestd node state poll: GET `/slurm/v0.0.38/node/{hostname}`, parse state field
- [x] 3.2 Implement poll loop with configurable interval (POLL_INTERVAL, default 5s) and timeout (POLL_TIMEOUT, default 30m)
- [x] 3.3 Detect drained states: "drained", "drained*", "down", "down*"
- [x] 3.4 Handle transient slurmrestd errors (log warning, retry on next interval)
- [x] 3.5 Exit code 2 on poll timeout

## 4. End-to-End Flow

- [x] 4.1 Wire main.go: validate env → connect MQ → discover hostname → publish allocation → poll loop → publish drained → exit 0
- [x] 4.2 Add signal handling (SIGTERM from Slurm scancel → clean exit)
- [x] 4.3 Write unit tests with mock MQ and mock HTTP for slurmrestd (test full lifecycle)
- [x] 4.4 Write integration test (gated by build tag): submit agent as Slurm job, verify MQ events received

## 5. Packaging

- [x] 5.1 Create `build/placeholder-agent.def` Singularity definition file (alpine base, copy binary, runscript)
- [x] 5.2 Create `build/build-placeholder-agent.sh`: compile static binary (CGO_ENABLED=0), build SIF if singularity available, warn if not
- [x] 5.3 Add `PLACEHOLDER_SIF_PATH` to daemon config struct
- [x] 5.4 Update daemon's SubmitPlaceholderJob to use `singularity run <SIF_PATH>` with `--export` env vars in the sbatch submission
- [x] 5.5 Document shared filesystem requirements and SIF deployment path in build script output
