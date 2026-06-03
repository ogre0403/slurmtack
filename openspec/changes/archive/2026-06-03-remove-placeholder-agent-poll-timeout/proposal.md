## Why

The placeholder agent currently enforces its own `POLL_TIMEOUT` and only treats exact state strings such as `drained` as terminal. That leaves `slurm_to_openstack` executions vulnerable to two failure modes: the job keeps polling even when Slurm has already moved the node into a drain-marked composite state like `MIXED+DRAIN`, and the agent can exit on its own wall clock instead of letting the placeholder job lifetime be governed by the Slurm partition policy.

## What Changes

- Remove the placeholder agent's internal overall drain poll timeout so the job no longer exits just because a local `POLL_TIMEOUT` budget elapsed
- Treat Slurm node state as a tokenized composite value during drain polling, and complete polling when the node carries a drain-complete token such as `drain`, `drained`, or `down`, including combined states such as `MIXED+DRAIN`
- Preserve the existing allocation-event and node-drained MQ flow while making the placeholder job runtime depend on Slurm job limits, cancellation, or successful drain completion
- Update placeholder-agent tests to cover composite drain states such as `MIXED+DRAIN` and the absence of a dedicated poll-timeout failure path

## Capabilities

### New Capabilities

- None

### Modified Capabilities

- `placeholder-agent-lifecycle`: change drain polling so it recognizes composite drain states such as `MIXED+DRAIN` and no longer enforces an agent-defined overall timeout

## Impact

- **Affected code**: `cmd/placeholder-agent`, related tests, and placeholder-agent lifecycle specs
- **Affected behavior**: placeholder jobs stop polling as soon as Slurm reports a drain-marked state for the allocated node, and otherwise continue until Slurm or an operator terminates the job
- **API surface**: None
- **Dependencies**: None
- **Breaking**: None
