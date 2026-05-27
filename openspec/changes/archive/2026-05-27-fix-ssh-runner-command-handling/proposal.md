## Why

The current SSH runner prepends `--execution-id` and `--step-name` to every remote command invocation. That breaks live commands such as `reboot` and the post-reboot `hostname` probe because the remote host receives arguments that are not part of the intended command payload, and operators cannot currently see the exact SSH command the daemon dispatched when diagnosing these failures.

## What Changes

- Stop mutating the remote command payload for reboot and SSH reachability probes with workflow metadata flags.
- Define a transport contract that preserves the intended command and arguments while still allowing execution metadata to be used for trace correlation.
- Emit execution-scoped trace logs before SSH dispatch that record the target host and the actual rendered remote command string.
- Add focused tests covering reboot invocation, reachability probes, and SSH command trace logging.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `ssh-reachability`: Reboot and reachability probe commands must execute the intended remote command without synthetic metadata flags being appended to the command line.
- `gpu-node-switch-observability`: Execution-scoped trace logs must include the concrete SSH command content and target used for remote runner dispatch.

## Impact

- Affected code: `internal/remote`, `internal/orchestrator`, and related tests.
- Affected behavior: reboot command dispatch, post-reboot SSH poll probes, and daemon trace logging for remote execution.
- Operational impact: debugging failed reboot and SSH poll flows becomes possible from daemon logs without reconstructing SSH command rendering manually.