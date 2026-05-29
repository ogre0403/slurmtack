## Context

The live `openstack_to_slurm` submission path accepts any request with a `node_name`, persists a new execution, and publishes MQ events so the orchestrator can run the full handoff workflow. There is no request-time guard for the case where the node is already back in an active Slurm state, so operators can accidentally submit the same ownership transition twice and trigger another destructive workflow run.

The codebase already has most of the signal needed to prevent that. `slurm.Client` exposes `GetNodeState`, and the attach-path classifier already distinguishes between nodes that still need `RESUME` (`drain`, `drained`, `down`) and nodes that are already schedulable in Slurm (`idle`, `alloc`, `mixed`). Reusing that distinction at request time gives us a concrete, minimal ownership guard without redesigning the downstream state machine.

## Goals / Non-Goals

**Goals:**
- Reject `openstack_to_slurm` requests before execution creation when the target node is already in an active Slurm state.
- Reuse a consistent Slurm-state classification so request admission and attach behavior agree on what counts as already under Slurm ownership.
- Keep the existing workflow unchanged for nodes that are not yet active in Slurm and still need the normal handoff.
- Add focused API and service tests that prove duplicate OpenStack-to-Slurm requests are blocked before persistence, MQ publish, and reboot-capable workflow steps.

**Non-Goals:**
- Redesigning the MQ-driven `openstack_to_slurm` state progression after a request is accepted.
- Introducing a full observed-owner model or new persisted ownership field in this change.
- Adding OpenStack-side request-time checks beyond the current workflow prechecks.
- Treating duplicate requests as a no-op success response; the requested behavior is explicit rejection.

## Decisions

### 1. Treat active Slurm attach states as the request-time ownership signal

The request guard should call `GetNodeState` for `openstack_to_slurm` submissions and classify the current state using the same token-based logic already used for attach safety. If the node is already schedulable in Slurm (`idle`, `alloc`, or `mixed` with no drain/down token), the request must be rejected as already owned by Slurm. If the node is in a resumable state (`drain`, `drained`, `down`), the request may proceed because the workflow may still need to finish bringing the node back into service. Unsupported or unreadable states should remain request-time errors rather than silently creating a new execution.

This keeps the guard anchored to a concrete signal the system already trusts during attach, and it matches the bug report: the unsafe case is a node that has already reached active Slurm service but is asked to switch to Slurm again.

Alternatives considered:
- Infer ownership from OpenStack compute-service status alone. Rejected because an OpenStack-disabled host does not prove whether Slurm has already resumed and taken ownership.
- Add a new persisted `observed_owner` field first. Rejected because it is larger than the requested fix and not required to stop the duplicate workflow.

### 2. Put the guard in the service layer, not the HTTP handler

The service layer already owns request validation, execution creation, and publish triggering. The new ownership check should live there as part of `RequestSwitch`, with the API handler translating the resulting validation-style error into a client-facing 4xx response.

This keeps the admission rule consistent for any caller that uses the service directly in tests or future entry points, and it avoids embedding Slurm policy in transport code.

Alternatives considered:
- Call Slurm directly from `internal/api`. Rejected because ownership admission is request semantics, not HTTP-specific behavior.
- Put the guard only in orchestrator precheck. Rejected because that is too late; the bug is that we create and admit a new execution at all.

### 3. Reuse the existing attach-state classifier from `internal/slurm`

The repository already has `ClassifyAttachState` and `EnsureNodeReadyForAttach` semantics that distinguish resumable, ready, and unsupported Slurm states. The request guard should reuse that classifier instead of creating a second list of special-case state strings.

Using one classifier reduces drift between request admission and attach execution. A node that would later be treated as already active by attach logic should be rejected at submission time for the same reason.

Alternatives considered:
- Add a second request-only classifier with slightly different state buckets. Rejected because it would create subtle mismatches around composite Slurm states.

### 4. Return a client-visible invalid-request error when the node is already Slurm-owned

When the request guard determines that the target node is already active in Slurm, `RequestSwitch` should fail with an `ErrInvalidSwitchRequest`-wrapped error that clearly states the node is already under Slurm ownership. The API handler should continue mapping that class of error to a client-facing rejection and must not persist an execution or publish MQ events.

This keeps the behavior aligned with other request-shape validation failures and gives operators an immediate explanation for why the request was refused.

Alternatives considered:
- Return HTTP 202 with no execution as a no-op. Rejected because it hides that the request was invalid and leaves callers unable to distinguish a real workflow from a refused duplicate.
- Return a generic 500 on Slurm state mismatch. Rejected because this is an admission decision, not an internal server failure.

## Risks / Trade-offs

- [A node in `drained` or `down` is treated as not-yet-owned by Slurm even if an operator thinks of it as "already in Slurm"] -> Keep the guard focused on the unsafe duplicate case: nodes already back in active Slurm service. The workflow may still legitimately need to resume non-active nodes.
- [Request admission now depends on live Slurm reachability] -> Surface Slurm lookup failures clearly and cover them in tests so operators can distinguish control-plane outages from duplicate-request rejection.
- [One shared classifier now drives both request and attach decisions] -> Reuse the existing helper and extend tests around representative state strings to reduce drift.

## Migration Plan

1. Wire a Slurm-backed ownership checker into `SwitchService` construction where a Slurm client is available.
2. Update `RequestSwitch` to perform the new guard only for `openstack_to_slurm` before execution persistence.
3. Keep existing request acceptance, persistence, and MQ publish behavior unchanged for all other valid cases.
4. Add service and API tests for rejected already-Slurm-owned requests, successful requests from resumable Slurm states, and lookup-failure behavior.
5. Deploy as a normal code rollout. After rollout, new duplicate OpenStack-to-Slurm submissions will be refused before they create executions. Rollback is a normal code rollback.

## Open Questions

- Should request-time Slurm lookup failures map to a 4xx validation-style rejection or a 5xx dependency error in the API layer? The current design keeps the ownership-match case as invalid input and leaves transport-level lookup failures to implementation choice.
