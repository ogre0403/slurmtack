## Context

The current switch daemon has three separate observability patterns:

- process-level `log.Printf` calls in `cmd/main.go`, `internal/mq`, and parts of `internal/orchestrator`
- persisted execution and step records in the store via `internal/engine.Runner`
- evidence files written by `internal/evidence.Writer`, currently used mainly by dry-run code

Those patterns are not aligned on the live workflow path. The orchestrator decides which action to run for each execution, calls control-plane and SSH operations directly, and advances states through `engine.Runner.Transition`, but most of those decisions and transitions emit no execution-scoped logs. `engine.Runner.RunStep` can persist step records, yet the runtime orchestrator mostly bypasses it, so step persistence and daemon logs diverge.

The design goal is to make the runtime path traceable without changing the state machine semantics from `docs/switch-design.md`. Operators need to see, in logs, which execution was selected, which workflow step started, what external action was attempted, what state transition was requested, whether the daemon is waiting on an asynchronous event, and how the execution reached a terminal outcome.

## Goals / Non-Goals

**Goals:**

- Emit execution-scoped structured logs for every daemon-managed workflow step in both switch directions.
- Cover the real control path: request creation, orchestrator action selection, external calls, state transitions, asynchronous waits, and terminal failure or completion.
- Reuse existing execution metadata so logs carry consistent fields such as execution ID, node name, direction, state, state version, and failure classification.
- Keep the change compatible with existing state machine tests and storage behavior.
- Make logging testable with focused unit and integration tests.

**Non-Goals:**

- Replacing persisted execution evidence with logs.
- Reworking the state machine, retry policy, or compensation rules.
- Introducing a third-party logging dependency.
- Logging secrets, full auth tokens, or arbitrary remote command payloads that may contain credentials.

## Decisions

### Decision: Use `log/slog` as the daemon’s structured logging API

The implementation will adopt the Go standard library `log/slog` package as the common API for daemon tracing. `cmd/main.go` will construct the base logger once and inject it into the orchestration path.

Rationale:

- Go 1.22 already supports `log/slog`, so no external dependency is needed.
- The workflow needs structured fields, not freeform strings, because operators will filter by `execution_id`, `node_name`, `state`, and `action`.
- Tests can capture structured output through a custom handler without scraping arbitrary message strings.

Alternatives considered:

- Continue using package-level `log.Printf`: rejected because interpolated strings make it hard to guarantee consistent fields or to assert coverage in tests.
- Adopt `zap` or `zerolog`: rejected because the repo does not currently depend on a logging framework, and the standard library already satisfies the requirement.

### Decision: Introduce a small execution-trace helper instead of logging ad hoc in every function

The daemon will use a shared helper that derives execution-scoped loggers and emits a stable event vocabulary. At minimum, log entries will carry:

- `component`
- `event`
- `execution_id`
- `node_name` when known
- `direction`
- `current_state`
- `state_version` when available
- `action`, `target_state`, or `step_name` when applicable

Representative event names:

- `request.accepted`
- `action.selected`
- `action.started`
- `action.succeeded`
- `action.failed`
- `transition.requested`
- `transition.succeeded`
- `transition.failed`
- `wait.entered`
- `wait.progress`
- `wait.satisfied`
- `execution.completed`
- `execution.failed`

Rationale:

- The user request is for logs on every workflow step, not a flood of line-by-line debug statements.
- A stable vocabulary makes trace output predictable across orchestrator, engine, MQ, and dry-run code.
- Derived execution loggers avoid repeatedly rebuilding the same field set.

Alternatives considered:

- Add independent `logger.Info(...)` calls everywhere with custom field names: rejected because drift between components would recreate the current observability gap.

### Decision: Instrument the existing runtime path directly before any larger handler refactor

This change will add tracing to the actual control path that runs today:

- `service.SwitchService.RequestSwitch` logs request acceptance and created execution metadata.
- `engine.Runner.Transition` logs transition intent, validation failures, and successful state advancement.
- `engine.Runner.FailExecution` logs failure classification and terminal-state selection.
- `engine.Runner.RunStep` logs step start and end so existing handler-based flows gain coverage automatically.
- `orchestrator.Orchestrator` logs action selection, action lifecycle, wait states, and terminal cleanup.
- `orchestrator.PollSSHReachable` logs polling entry, retry progress, success, and timeout.
- `mq.Consumer` logs allocation and drained events with execution-scoped fields as they advance runtime state.

The change will not require the orchestrator to route every action through `RunStep` immediately. Instead, it will trace the current methods directly and reuse `RunStep` logging where handlers already exist.

Rationale:

- This is the smallest change that closes the real debugging gap.
- A full workflow refactor to force every action through step handlers is broader than the user asked for and would increase delivery risk.
- Direct instrumentation still leaves room for a future refactor where more orchestrator actions become step handlers.

Alternatives considered:

- Rewrite orchestrator actions to use only `RunStep` before adding logs: rejected as too large for the stated problem.

### Decision: Define coverage by workflow step, including asynchronous waits

The implementation will consider each step from `docs/switch-design.md` covered only when the daemon emits logs for both entry and outcome. Coverage includes:

- request accepted
- placeholder submission and transition to `awaiting_source_allocation`
- allocation event consumed and transition to `node_identified`
- lease acquisition
- precheck start and result
- source quiesce start and result
- drained wait entered and drained event consumed
- source detach transition
- host reconfiguration start and result
- reboot request and expected disconnect boundary
- SSH reachability polling progress and success or timeout
- target attach start and result
- verification start and result
- lease release and terminal completion or failure

The design treats asynchronous waits as first-class trace steps because those are the longest silent periods during a real switch.

Rationale:

- Silent waiting is currently the hardest case to debug.
- The design document explicitly models placeholder allocation and drained confirmation as workflow steps, so the trace model must match.

Alternatives considered:

- Log only mutating operations: rejected because operators also need to see why the daemon is idle or blocked.

### Decision: Keep sensitive values out of logs and prefer identifiers over raw payloads

Logs will record identifiers and summarized outcomes, not secrets or full request bodies from external systems. For example, logs may include `job_id`, `node_name`, `execution_id`, or HTTP status, but must not emit API tokens, SSH credentials, or raw OpenStack auth data.

Rationale:

- The daemon runs in operational environments where log destinations may be broader than the state store.
- Traceability is useful only if operators can safely enable and retain the logs.

Alternatives considered:

- Log full request and response payloads everywhere: rejected because it creates secret-handling risk and duplicates evidence capture responsibilities.

## Risks / Trade-offs

- [Increased log volume during polling and retries] → Throttle wait-progress logs to meaningful intervals and keep fields structured so downstream filters can suppress noise.
- [Cross-cutting constructor churn] → Add logger parameters incrementally from `cmd/main.go` into the runtime path instead of rewriting unrelated packages.
- [Mixed logging styles during migration] → Use `slog` for newly instrumented workflow code first and leave unrelated package-level logs unchanged until they can be migrated cleanly.
- [False sense of coverage if only messages change] → Add tests that assert required events and fields for representative success and failure flows.

## Migration Plan

1. Add a base `slog.Logger` in `cmd/main.go` and thread it into `service`, `engine`, `orchestrator`, and `mq` constructors that participate in switch execution.
2. Add the execution-trace helper and instrument `RequestSwitch`, `Transition`, `FailExecution`, `RunStep`, and orchestrator action methods.
3. Instrument MQ event handling and SSH polling so allocation waits and reboot waits no longer go silent.
4. Update focused tests to capture logger output and assert that representative workflows emit the required events and execution fields.
5. Roll back by reverting logger injection and helper usage if necessary; this change does not require a database or protocol migration.

## Open Questions

- Should trace logs also be mirrored into `events.jsonl` through `internal/evidence.Writer`, or is process log output sufficient for this change?
- Should repeated wait-progress entries be emitted at every poll attempt or at a coarser interval to reduce noise?
- Do we want a dedicated `trace_id` or is `execution_id` sufficient as the primary correlation key for now?