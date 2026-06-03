## Context

The current placeholder-agent drain loop has two mismatches with the rest of the system. First, `pollDrainLoop` creates `time.After(cfg.PollTimeout)` and returns exit code 2 when that local deadline expires. That makes the agent impose its own runtime budget even though placeholder jobs are already subject to Slurm job limits defined by partition and scheduler policy. Second, `getNodeState` joins Slurm state tokens with `+`, but the drain check only does exact string lookup against `drained`, `drained*`, `down`, and `down*`. The observed `slurm-14.out` run shows the practical failure mode: the node stayed in `MIXED+DRAIN` until the local timeout fired, so the agent never published `execution.drained`.

This change is small in surface area but cross-cutting enough to merit a design note because it alters documented lifecycle behavior, the agent's exit-code contract, and the test strategy around Slurm node-state interpretation.

## Goals / Non-Goals

**Goals:**

- Remove the placeholder-agent's internal overall poll timeout
- Treat Slurm node state token-by-token so composite drain-marked states complete the poll loop
- Keep the existing MQ contract unchanged: allocation event first, drained event after drain completion
- Make placeholder job lifetime depend on Slurm job policy rather than agent wall-clock logic
- Update unit and integration coverage to reflect the new polling contract

**Non-Goals:**

- Moving drain confirmation from the placeholder agent into the daemon
- Changing placeholder job submission semantics beyond continuing to rely on Slurm partition/job policy for runtime limits
- Adding new API fields, scheduler metadata lookups, or operator-configurable agent-side timeout behavior

## Decisions

### Remove the agent-defined overall poll deadline

**Choice:** Delete `POLL_TIMEOUT` handling from the placeholder agent and run the poll loop until one of three conditions occurs: drain completion is observed, the process context is cancelled, or Slurm/job control terminates the job externally.

**Alternatives considered:**

- Keep `POLL_TIMEOUT` but raise the default: rejected because it still duplicates scheduler policy and can still expire before the partition's intended limit
- Keep `POLL_TIMEOUT` as an optional override: rejected because the requested operational model is explicit that timeout ownership belongs to Slurm partition policy, not the job program

**Rationale:** The placeholder job already lives inside Slurm. Letting Slurm own job lifetime avoids split timeout sources and removes a class of false failures where the agent exits even though the partition would have allowed more wait time.

### Classify drain completion from state tokens instead of exact strings

**Choice:** Parse Slurm node state as case-insensitive `+`-delimited tokens and treat the poll as complete when the state includes any drain marker that means the node has entered Slurm drain control for the switch window. At minimum, `drain`, `drained`, `drained*`, `down`, and `down*` must satisfy completion even when returned inside a composite state string such as `MIXED+DRAIN`.

**Alternatives considered:**

- Keep exact string matching and add a few more composite literals: rejected because Slurm state combinations are open-ended and the literal list will keep drifting
- Wait only for `drained` or `down` tokens: rejected because the observed `MIXED+DRAIN` case can persist while the placeholder job itself is still the remaining workload on the node, which prevents the required MQ handoff

**Rationale:** The rest of the codebase already reasons about Slurm state token-by-token for attach/resume safety. Applying the same pattern here fixes the observed `node already drain but still polling` symptom from `slurm-14.out` without overfitting to a single uppercase literal.

### Simplify the exit-code contract

**Choice:** Remove the dedicated poll-timeout exit path and keep only startup/config failures and MQ publish failures as agent-defined non-zero exits.

**Alternatives considered:**

- Preserve exit code 2 for some synthetic local timeout: rejected because the change intentionally removes that failure mode
- Repurpose exit code 2 for composite-state parsing failures: rejected because parse/classification problems should surface through normal startup/runtime errors, not a misleading timeout code

**Rationale:** Once the agent no longer owns an overall deadline, a dedicated timeout exit code becomes misleading and encourages downstream handling that assumes a local timeout still exists.

## Risks / Trade-offs

- [A placeholder job can now poll indefinitely when Slurm never reaches a drain-marked state and the partition has no effective wall-time limit] -> Mitigation: rely on scheduler policy for production partitions and keep operator cancellation as the control path for abnormal waits.
- [Publishing `execution.drained` when Slurm reports `MIXED+DRAIN` could advance while unrelated user jobs still remain] -> Mitigation: scope the change to the placeholder-job-controlled switch window that already owns the target node, and add focused tests/documentation for the exact `MIXED+DRAIN` regression being fixed.
- [Existing automation may still expect exit code 2] -> Mitigation: update specs and tests together so the new contract is explicit before implementation lands.

## Migration Plan

1. Update the placeholder-agent lifecycle spec and tests first so the new contract is explicit.
2. Remove `POLL_TIMEOUT` parsing and deadline logic from `cmd/placeholder-agent`.
3. Add token-based drain-state classification and regression tests for `MIXED+DRAIN` plus other supported composite states.
4. Rebuild and redeploy the placeholder-agent binary/container so newly submitted placeholder jobs pick up the new behavior.
5. If rollback is needed, restoring the previous binary restores the previous local-timeout behavior for newly submitted jobs; already running jobs keep the binary they started with.

## Open Questions

None.
