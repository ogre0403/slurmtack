## Context

The Slurm submission layer already understands partitions: `slurm.PlaceholderJobRequest` has a `Partition` field, and the slurmrestd client emits `#SBATCH --partition=...` when that field is set. The missing behavior is upstream. The switch request model, execution record, and placeholder-submission call sites currently carry only the requested Slurm constraint, so a caller cannot select a partition even though the client can submit one.

This change crosses the API, service, domain, store, and orchestration layers. The execution record must persist the requested partition because placeholder submission is asynchronous and may happen after a restart. That persistence requirement matters most for SQLite: the current store initialization executes the embedded schema but does not automatically evolve existing tables, so adding a new execution field needs an explicit compatibility step.

## Goals / Non-Goals

**Goals:**

- Accept an optional `slurm_partition` on `POST /v1/switches` for `slurm_to_openstack` requests
- Persist the requested partition on the execution in both memory and SQLite stores
- Pass the requested partition to placeholder submission from every call site that creates the Slurm job
- Preserve current behavior when callers omit `slurm_partition`
- Add focused tests for API binding, execution persistence, and placeholder submission with and without a partition

**Non-Goals:**

- Partition selection for `openstack_to_slurm` workflows
- Automatic partition discovery or validation against live Slurm metadata
- Changing the requested partition after an execution has been accepted
- Expanding execution status or list responses unless implementation work proves it is necessary for debugging

## Decisions

### Add an optional `slurm_partition` request field

**Choice:** Extend the existing switch request payload with an optional `slurm_partition` string.

**Alternatives considered:**

- Global configuration such as `SLURM_PARTITION`: too coarse because different switch requests may need different partitions
- Overloading `slurm_constraint`: incorrect because Slurm constraints and partitions are separate selectors with different scheduler semantics

**Rationale:** The current API already accepts per-request Slurm placement data through `slurm_constraint`. Adding a sibling `slurm_partition` field is the smallest, most explicit extension and matches the current user intent: choose the partition for a specific placeholder job.

### Persist partition on the execution record

**Choice:** Add `RequestedSlurmPartition` to `domain.Execution` and thread it through the service layer plus both store implementations.

**Alternatives considered:**

- Pass the partition directly from API to Slurm submission without storing it: not viable because the workflow is asynchronous and placeholder submission may happen after process restart
- Store request metadata in an opaque JSON blob: adds complexity without helping the current query patterns or tests

**Rationale:** Execution state is already the durable handoff between request acceptance and later orchestration work. Partition belongs in that state for the same reason constraint does.

### Use an idempotent SQLite compatibility step

**Choice:** Keep `schema.sql` up to date for new databases and add startup logic that detects whether `executions.requested_slurm_partition` exists, adding it when missing.

**Alternatives considered:**

- Requiring operators to recreate the database: unacceptable because it loses in-flight and historical execution data
- Introducing a full migration framework for a single column addition: disproportionate to the scope of this change

**Rationale:** The repository currently bootstraps SQLite with embedded SQL only. A targeted, idempotent compatibility step keeps existing deployments working without introducing a larger migration system.

### Thread partition through both placeholder submission paths

**Choice:** Update every path that calls `SubmitPlaceholderJob`, including the orchestrator and the allocation handler.

**Alternatives considered:**

- Updating only the orchestrator path: incomplete because tests and integration helpers also use the allocation handler directly

**Rationale:** The repository has more than one entrypoint into placeholder submission. They must stay behaviorally aligned or the new field will work in one path and silently disappear in another.

### Leave partition syntax validation to Slurm

**Choice:** Treat `slurm_partition` as an optional opaque string and let slurmrestd reject invalid partition names.

**Alternatives considered:**

- Enforcing local format rules: brittle because valid partition names are cluster-specific
- Querying Slurm for partition existence during request acceptance: adds latency and a new dependency to the API path

**Rationale:** The daemon already surfaces Slurm submission failures. Reusing that path keeps request handling fast and avoids baking cluster policy into the API layer.

## Risks / Trade-offs

- Existing SQLite databases need a schema evolution step on startup -> Mitigation: detect the missing column before use and apply an idempotent `ALTER TABLE` path covered by store tests.
- Invalid partition names will fail only when the placeholder job is submitted -> Mitigation: preserve the Slurm API error details so operators can see the exact rejection.
- Status responses may not expose the requested partition, which can make debugging slightly less direct -> Mitigation: keep the field persisted on the execution and add response exposure later only if operators need it.

## Migration Plan

1. Add `requested_slurm_partition` to `schema.sql` for fresh databases.
2. Update SQLite startup to detect and add the missing column for existing databases before any execution reads or writes.
3. Deploy the new daemon version before sending requests that include `slurm_partition`.
4. If rollback is needed, the previous binary can still run against the expanded table because its SQL selects explicit columns and ignores unknown ones.

## Open Questions

None.