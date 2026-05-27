## Context

`remote.CommandRequest` currently carries both the remote command payload (`Command` and `Args`) and workflow correlation metadata (`ExecutionID` and `StepName`). `SSHRunner.Execute` collapses those concerns by prepending `--execution-id` and `--step-name` to every SSH invocation before passing control to `ExecSSHExecutor`. That makes raw system commands such as `reboot` and `hostname` incorrect on the target host, and the same mutation can affect other direct SSH operations such as reboot diagnostics.

The transport layer already owns SSH target selection, option rendering, and shell quoting in `internal/remote/ssh_executor.go`, but it does not expose the rendered remote command anywhere in daemon logs. When reboot or reachability fails, operators can see that SSH failed, but not which concrete command string was sent.

This change spans the remote transport boundary and the orchestrator call sites that populate execution metadata, so a short design is useful before implementation.

## Goals / Non-Goals

**Goals:**

- Preserve the exact remote command payload for reboot, reachability probes, and other SSH-run commands.
- Keep execution correlation metadata available for local tracing without pushing that metadata onto the remote shell command line.
- Emit a structured pre-dispatch log that records the target host and rendered remote command string.
- Add focused tests that cover payload preservation, rendered command logging, and the reachability probe path.

**Non-Goals:**

- Introducing a remote wrapper script or a new command-line protocol on the target hosts.
- Changing SSH authentication, timeout defaults, or the orchestrator state machine.
- Logging private key paths, raw SSH options, or other local transport details that are not needed to debug the remote payload.
- Refactoring unrelated remote runner implementations.

## Decisions

### Decision: Treat command payload and workflow metadata as separate concerns

`CommandRequest.Command` and `CommandRequest.Args` will become the complete remote payload contract. `ExecutionID` and `StepName` remain on the request, but they are correlation metadata only. The SSH runner must pass the payload through unchanged and must not synthesize additional remote arguments from the metadata.

Rationale:

- The current bug exists because the transport mutates payload at the last moment.
- Reboot, `hostname`, and diagnostics commands are ordinary system commands; they cannot safely accept daemon-specific flags.
- Keeping metadata on the request still allows logging and correlation without forcing broad call-site churn.

Alternatives considered:

- Special-case commands such as `reboot` and `hostname` so only some invocations skip metadata injection: rejected because it is brittle and leaves the same bug available for future commands.
- Remove metadata from `CommandRequest` entirely: rejected because callers still need a place to attach correlation fields for logging.

### Decision: Pass the full `CommandRequest` through the SSH executor boundary

The `SSHExecutor` boundary will operate on a `CommandRequest` (or an equivalent internal request struct) instead of only `(host, command, args, timeout)`. That lets the transport render the remote command once, use the same rendering for `exec.CommandContext`, and emit logging with the request metadata that came from the caller.

Rationale:

- Rendering and process execution already live in `ExecSSHExecutor`; the same layer should own the exact command string that gets logged.
- Passing the full request avoids duplicating command rendering logic across `SSHRunner` and `ExecSSHExecutor`.
- Tests can assert one rendering path for both process arguments and logs.

Alternatives considered:

- Reconstruct the rendered command separately in `SSHRunner` just for logging: rejected because it creates drift between what is logged and what is executed.
- Log only the high-level `Command` field without rendered arguments: rejected because it would not expose the bug the user reported.

### Decision: Populate reachability probes with correlation metadata explicitly

`doReboot` already sets `ExecutionID` and `StepName`. The post-reboot probe path will do the same by passing execution metadata and a stable step name such as `ssh_probe` into the `hostname` request. That preserves execution-level correlation for dispatch logs while keeping the remote command payload as plain `hostname`.

Rationale:

- The current poll path sends empty metadata fields, which still become empty synthetic flags today.
- Once metadata stops mutating payload, explicit probe metadata becomes the simplest way to keep logs attributable to a specific execution.
- A stable probe step name makes test expectations and operational filtering predictable.

Alternatives considered:

- Accept uncorrelated probe logs: rejected because post-reboot failures are exactly the path operators need to trace back to an execution.

### Decision: Log only the rendered remote payload and target identity

The SSH dispatch trace will record the resolved SSH target and the rendered remote command string before `ssh` starts. It will omit local-only transport details such as identity file paths and raw `-o` values.

Rationale:

- The debugging need is to confirm the command payload sent to the remote shell.
- Local transport settings may be noisy or sensitive, while the rendered remote payload is the minimum data needed to diagnose this class of bug.

Alternatives considered:

- Log the entire local `ssh` argv, including options and key paths: rejected because it increases leakage risk without materially improving diagnosis of payload corruption.

## Risks / Trade-offs

- [Risk] Changing the executor boundary can ripple into local tests and fakes. -> Mitigation: keep the public `remote.Runner` interface stable and confine the signature change to the transport layer.
- [Risk] Logging remote command strings can expose sensitive arguments if future callers pass secrets over SSH. -> Mitigation: keep this change focused on current reboot, probe, and diagnostics paths and document that secret-bearing commands require redaction before logging.
- [Risk] The chosen probe step name becomes part of the operator-facing vocabulary. -> Mitigation: use one stable name and assert it in focused tests.

## Migration Plan

No data migration is required. Deploy the code change with focused unit coverage for `internal/remote` and orchestrator reachability. If rollback is needed, revert the transport refactor and dispatch logging together so payload rendering and log behavior stay aligned.

## Open Questions

None.