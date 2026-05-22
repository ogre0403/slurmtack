## Context

The repository currently has only a minimal Go entrypoint in [cmd/main.go](/workspaces/slurmtack/cmd/main.go), so this change establishes the behavioral contract for a new switch daemon rather than modifying an existing orchestration service. The design source in [docs/switch-design.md](/workspaces/slurmtack/docs/switch-design.md) already defines the target workflow, failure domains, state machine, and observability constraints; this document narrows those ideas into implementation decisions that can be carried into code.

The switch crosses multiple authority boundaries: Slurm decides node allocation and drain state, OpenStack decides compute ownership, and host-local mutation must happen through constrained SSH operations. The daemon therefore needs durable execution state, versioned transitions, and enough evidence capture to debug failures at the reboot boundary.

## Goals / Non-Goals

**Goals:**
- Define a daemon architecture that can accept asynchronous switch requests and drive the workflow to completion or an auditable terminal failure.
- Separate execution lifecycle, node binding, and per-node lease handling so Slurm-to-OpenStack requests can start before a concrete node is known.
- Standardize how remote actions, control-plane calls, events, and verification results are persisted for replay and debugging.
- Establish a spec contract that implementation can satisfy in incremental tasks.

**Non-Goals:**
- Choosing a specific storage engine, message broker topology, or OpenStack SDK.
- Defining UI workflows or operator dashboards.
- Making multi-node switching atomic across several hosts.
- Encoding every host-specific GPU mode toggle before hardware details are known.

## Decisions

### 1. Model the daemon as a persisted state machine

The daemon will treat the node switch state machine as the source of truth and persist every transition with a monotonic `state_version`. This is preferable to a workflow assembled from ad hoc retries because the reboot boundary, duplicate events, and compensation logic all require explicit checkpoints.

Alternative considered: derive workflow state only from logs and in-flight goroutines. Rejected because it makes duplicate delivery handling, operator debugging, and post-crash recovery non-deterministic.

### 2. Split request execution from node lease acquisition

Each request creates an execution record immediately, even when `node_name` is still unknown. For Slurm-to-OpenStack, the daemon binds the execution to a node only after the placeholder job publishes the allocation event, then acquires the per-node lease before any node-bound mutation starts.

Alternative considered: require callers to identify the target node up front. Rejected because the design explicitly requires Slurm to choose the eligible GPU node.

### 3. Use a command-wrapper remote runner instead of general SSH automation

All host mutation will flow through a fixed command wrapper invoked via SSH, with `execution_id` and `step_name` attached to every call and structured JSON returned for audit. This keeps OS-level changes constrained while still allowing the daemon to capture command duration, exit code, and structured snapshots.

Alternative considered: direct shell command execution or Ansible-based orchestration. Rejected because the design requires controlled SSH operations, non-interactive safety, and deterministic audit trails.

### 4. Treat message-bus events as versioned inputs, not as truth

Allocation events, drained notifications, and host-reachable signals will be accepted only when `execution_id` and `state_version` match the active execution. The daemon will persist the resulting transition and continue to verify the relevant control-plane or host state before moving forward.

Alternative considered: trust asynchronous events as authoritative. Rejected because delayed or duplicated events are expected failure modes and must not mutate the wrong execution.

### 5. Make observability a first-class data model

Execution records, step records, snapshots, API request or response captures, and deterministic log directories will be part of the primary design instead of an implementation afterthought. This allows operators to answer which step failed, what changed, and whether rollback remains safe without reconstructing history from mixed logs.

Alternative considered: rely on aggregated daemon logs only. Rejected because reboot and partial-mutation failures need structured per-execution evidence.

## Risks / Trade-offs

- [Slurm allocation event never arrives] → Time out `awaiting_source_allocation`, capture placeholder job diagnostics, and fail without taking a node lease.
- [Execution state and observed control-plane state diverge] → Re-verify source and target ownership before every irreversible transition and require version checks on events.
- [Host mutation command wrapper is too narrow for real hardware differences] → Keep wrapper verbs fixed but allow host-specific implementation behind each validated verb.
- [Reboot returns with unknown host state] → Treat reboot as a checkpoint, require boot-ID comparison, and escalate to `failed_manual_recovery` when health cannot be proven.
- [Early implementation overfits to one direction] → Keep shared execution, lock, logging, and verification abstractions direction-agnostic, then plug in source and target specific steps.

## Migration Plan

1. Introduce domain models for executions, step records, node leases, and state transitions.
2. Implement request acceptance, state persistence, and status reporting without host mutation.
3. Add the Slurm placeholder-job allocation path and event correlation.
4. Add remote-runner, precheck, detach, attach, and verification step handlers behind interfaces.
5. Add log-manifest writing and structured evidence capture before enabling real host mutation in non-test environments.
6. Roll out with dry-run or simulated runners first, then enable live mutation per environment.

Rollback strategy: if deployment introduces daemon defects, disable new switch requests and leave existing execution evidence intact. Automatic rollback of in-flight node switching remains step-scoped and is governed by the workflow requirements rather than by deployment tooling.

## Open Questions

- Which exact wrapper commands are needed to toggle GPU mode on the target hardware platform?
- Is reboot mandatory for both directions or only for specific host reconfiguration paths?
- Should host-reachable detection be MQ-driven, SSH-polled, or a hybrid approach?
- Which inventory source is authoritative for node identity, management IP, and ownership metadata?
- What timeout and retry defaults should be assigned to each non-terminal state?