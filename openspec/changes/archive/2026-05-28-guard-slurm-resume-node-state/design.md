## Context

The live `openstack_to_slurm` attach path currently calls `ResumeNode` without first checking the node's Slurm state. Real `slurmrestd` only accepts `RESUME` when the node is in a resumable state such as drain or down, so nodes that are already active can fail with `slurm_update error: Invalid node state specified` even though no resume is needed.

The codebase already has the primitives needed to fix this cleanly. `slurm.Client` exposes `GetNodeState`, the Slurm verification path already recognizes active scheduler states such as `idle`, `alloc`, and `mixed`, and the integration tests show that Slurm can return composite state strings like `drained+drain` and `idle+drain`.

## Goals / Non-Goals

**Goals:**
- Guard OpenStack-to-Slurm target attachment by classifying the current Slurm node state before issuing `ResumeNode`.
- Handle composite Slurm states consistently so the attach step recognizes drain/down flags even when they appear together with another state token.
- Preserve successful progression for nodes that are already schedulable by skipping an unnecessary `RESUME` call.
- Add focused regression coverage around guarded resume decisions in both live orchestration and reusable attach-step code.

**Non-Goals:**
- Change the slurmrestd REST surface, authentication model, or request payloads.
- Redesign downstream verification semantics beyond the attach precondition.
- Model every possible Slurm state transition in the system; this change only defines the states relevant to attach safety.

## Decisions

### Add a shared attach-state classifier

The attach behavior should be driven by a small shared classifier that interprets the normalized Slurm state string as tokens split on `+`. That classifier should return one of three outcomes: resume required, resume not required, or attach must fail.

This keeps `internal/orchestrator` and `internal/slurm` aligned. The orchestrator currently performs the live attach directly, while the engine package also exposes a reusable `slurm.AttachHandler`. If the guard lives in only one place, the other path will remain behaviorally incorrect.

Alternative considered: put the guard only in `doAttach`. Rejected because tests and any engine-driven attach flow would still be able to issue invalid `RESUME` requests.

### Only resume drain/down states; skip already schedulable states

The classifier should treat any state containing `drain`, `drained`, or `down` as resumable. It should treat active states such as `idle`, `alloc`, or `mixed` as already schedulable only when no drain/down token is present. For already schedulable states, attach becomes a no-op and the workflow continues to verification without calling `ResumeNode`.

If the state is neither resumable nor already schedulable, the attach step should fail explicitly without calling `ResumeNode`. That avoids masking real node-health problems behind a generic slurmrestd mutation error.

Alternative considered: always call `ResumeNode` and rely on best-effort idempotent error parsing. Rejected because the real failure mode is the avoidable `Invalid node state specified` API error.

### Reuse the existing Slurm client interface

Implementation should call `GetNodeState` and then decide whether to call `ResumeNode`; the `slurm.Client` interface does not need a new method. This keeps workflow policy in the orchestration/step layer and leaves the HTTP client responsible only for Slurm API operations.

Alternative considered: add a higher-level `EnsureNodeActive` or `ResumeNodeIfNeeded` client method. Rejected because it would move orchestration policy into the transport client and require broader interface changes.

### Tighten tests and fakes around real Slurm semantics

The focused tests should cover at least three cases: a composite drain state that must resume, an already active state that must skip resume, and an unsupported state that must fail before issuing a mutation. Any fake Slurm client used by attach tests should behave closely enough to real Slurm to expose invalid unconditional resumes.

Alternative considered: keep existing permissive fakes and only add unit tests for a helper. Rejected because the current permissive behavior is part of how this regression slipped through.

## Risks / Trade-offs

- Slurm state strings can vary and may include tokens not covered by today's tests. → Mitigation: classify by tokens instead of exact whole-string matches and add representative composite-state cases.
- Treating non-resumable, non-schedulable states as hard errors may fail some executions earlier than before. → Mitigation: prefer explicit attach failure over an invalid mutation request, and keep the error message tied to the observed node state.
- Sharing decision logic across orchestrator and step handlers adds a small abstraction. → Mitigation: keep the helper narrow and scoped only to resume-decision classification.

## Migration Plan

No schema, API, or deployment migration is required. After rollout, newly started executions and any in-flight `openstack_to_slurm` executions that reach target attachment will apply the guarded resume behavior on their next attach attempt. Rollback is a normal code rollback.

## Open Questions

- None for this change.