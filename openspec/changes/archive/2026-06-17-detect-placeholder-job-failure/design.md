## Context

The current `slurm_to_openstack` flow submits a placeholder job, records `placeholder_job_id`, transitions the execution to `awaiting_source_allocation`, and then stops orchestrator progression until MQ delivers an allocation event. That model assumes the placeholder agent will always start far enough to publish `execution.allocation`.

In practice, the placeholder job can reach a terminal Slurm state before the agent publishes anything, for example when the resolved SIF path is unreadable on the allocated node or the job exits immediately during startup. In that case:

- the execution remains `active` in `awaiting_source_allocation`
- the running `wait_for_source_allocation` step never closes
- the daemon has no recovery path other than manual operator inspection
- the dashboard continues to show stale waiting information

The current codebase already has most of the pieces needed for a minimal fix:

- a durable execution model that can fail from `awaiting_source_allocation` to `failed_non_destructive`
- step tracking with `error_summary`
- Slurm REST integration for submit/cancel operations
- dashboard polling for the execution list

What is missing is a daemon-side source of truth for placeholder job terminal state during the allocation wait.

## Goals / Non-Goals

**Goals:**

- Detect placeholder jobs that terminate before publishing an allocation event.
- Fail the execution promptly from `awaiting_source_allocation` instead of waiting forever.
- Persist a readable operator-visible reason on both the wait step and the execution.
- Keep the dashboard detail panel aligned with the updated execution outcome.
- Preserve the existing successful MQ-driven allocation path.

**Non-Goals:**

- Replacing RabbitMQ allocation events with polling-only node binding.
- Changing the public REST response shape for execution detail or step history.
- Introducing a new placeholder-agent failure event or changing the agent lifecycle contract.
- Building a general timeout framework for all wait states beyond this specific placeholder failure gap.

## Decisions

### 1. Monitor placeholder job state from the orchestrator while waiting for allocation

`awaiting_source_allocation` will stop being a pure no-op state. The orchestrator will keep a worker alive for that state and poll Slurm for the recorded `placeholder_job_id`.

This fits the current architecture better than adding a separate watcher because the orchestrator already owns execution progression, restart recovery, and terminal failure classification.

Alternatives considered:

- Timeout only: rejected because it still leaves operators waiting long after Slurm already knows the job failed.
- New MQ failure event from the placeholder agent: rejected because the failing case includes jobs that never start the agent process at all.
- Shelling out to Slurm CLI tools: rejected because the daemon already standardizes on slurmrestd for control-plane integration.

### 2. Add a Slurm client job-state read API that uses workload identity

The Slurm client will gain a job-state lookup method using the documented job-state endpoint relative to `SLURM_API_URL`. The read will use the execution's workload identity, matching the credentials used for job submission, so the daemon observes the job from the same security context that created it.

The client response should normalize at least:

- raw Slurm state text
- whether the job is terminal
- whether the terminal state is a failure for allocation waiting

Alternatives considered:

- Using admin credentials for job-state reads: rejected because the submission already belongs to the workload identity and the same identity should be sufficient for observing its own placeholder job.
- Returning raw JSON from slurmrestd directly to the orchestrator: rejected because the orchestration logic should not need to understand slurmrestd wire format.

### 3. Treat any terminal pre-allocation job outcome as a non-destructive execution failure

While the execution has not yet bound a node, a placeholder job that ends before allocation is a pre-mutation failure. The orchestrator will therefore fail the execution as `failed_non_destructive`.

This includes:

- explicit Slurm failure states such as `FAILED`, `BOOT_FAIL`, `CANCELLED`, `TIMEOUT`, or `NODE_FAIL`
- unexpected terminal states such as `COMPLETED` before any allocation event was observed

The running `wait_for_source_allocation` step will be closed as `failed`, and both the step and execution will store a concise summary that names the job ID and the observed state.

Alternatives considered:

- Leaving `COMPLETED` as success: rejected because node binding never happened, so the switch cannot safely continue.
- Reclassifying the failure as rollback-required: rejected because no node-bound mutation has started yet.

### 4. Keep MQ as the only success path for binding the node

Polling job state is only a failure detector. A successful switch still requires the existing `execution.allocation` MQ event to provide the bound node name and advance the execution to `node_identified`.

This keeps the current node-binding contract intact and limits the change to the missing negative path.

### 5. Refresh the selected dashboard detail view on the existing polling cadence

The dashboard already refreshes execution lists periodically, but the selected drilldown panel stays stale until the operator reopens it. The dashboard will reuse the same polling cadence to refresh the selected execution summary and step timeline when a selection is active.

This avoids any API changes and ensures that a newly failed allocation wait becomes visible where the operator is already looking.

## Risks / Trade-offs

- [Extra slurmrestd traffic while waiting for allocation] -> Mitigation: poll only active `awaiting_source_allocation` executions and reuse the existing orchestrator tick interval rather than introducing a tighter loop.
- [Slurm installations may report different terminal state strings] -> Mitigation: preserve raw state text, normalize known terminal outcomes explicitly, and treat any terminal pre-allocation outcome as failure unless an allocation event has already advanced the execution.
- [Race between a late allocation MQ event and a near-simultaneous failure poll] -> Mitigation: keep existing state-version and unexpected-state guards so whichever path wins first makes the other path a harmless no-op.
- [Dashboard and daemon may deploy out of sync] -> Mitigation: backend failure detection stands on its own, and the dashboard change is additive because it reuses existing detail endpoints.

## Migration Plan

No datastore migration is required because the execution and step models already contain the fields needed for terminal failure state, `final_error_summary`, and step-level `error_summary`.

Deployment steps:

1. Deploy the daemon with the new Slurm job-state polling logic.
2. Deploy the dashboard refresh change.
3. Allow the orchestrator to recover any existing active executions; `awaiting_source_allocation` executions will resume monitoring after restart instead of staying orphaned.

Rollback strategy:

- Revert the daemon and dashboard together. Existing persisted failure summaries and failed steps remain readable because they use existing fields.

## Open Questions

None. The change can proceed with the documented Slurm job-state endpoint and the existing execution failure model.
