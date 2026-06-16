## Context

The current dashboard inventory path discovers all Slurm partitions from `ListPartitions`, returns all discovered nodes, and lets the browser choose either one partition or an "All partitions" view. The dashboard runtime config already exists, but today it only publishes safe SIF-path hint fields. On the write path, `slurm_to_openstack` accepts an optional `slurm_partition`, while `openstack_to_slurm` validates only Slurm ownership state and does not check partition membership.

The requested behavior is an optional deployment-level scope: when `SLURM_CLOUD_PARTITION` is unset, nothing changes; when it is set, both visibility and allowed actions must collapse to that one partition. Because the dashboard is a static nginx-served UI, the same configured value also has to be published through the existing runtime config whitelist so the browser can render the correct fixed-partition experience without introducing another API.

## Goals / Non-Goals

**Goals:**

- Add one optional `SLURM_CLOUD_PARTITION` configuration source that defaults to the current unscoped behavior.
- Enforce the partition scope in backend inventory and switch admission logic, not only in the browser.
- Expose the configured partition through safe runtime config so the dashboard can remove non-applicable partition choices.
- Keep the existing API shapes and runtime config pattern instead of introducing new endpoints.

**Non-Goals:**

- Adding multi-partition allowlists or per-user authorization rules.
- Reworking the dashboard inventory model beyond the new optional single-partition scope.
- Validating the configured partition against live Slurm at process startup.

## Decisions

### Decision: Treat `SLURM_CLOUD_PARTITION` as one optional shared deployment setting

`internal/config.Config` will gain a `SlurmCloudPartition` field loaded from `SLURM_CLOUD_PARTITION`. The same env var will be wired into the nginx container so its startup script can emit `slurmCloudPartition` inside `window.SLURMTACK_CONFIG`.

This keeps one source of truth for both daemon enforcement and UI rendering. Leaving the value unset preserves the current discovery-driven behavior without a separate feature flag.

**Alternatives considered**

- Add a UI-only env var in nginx: rejected because the backend must enforce the same scope for non-browser callers.
- Add separate daemon and dashboard config names: rejected because split configuration would drift easily and create ambiguous behavior.

### Decision: Enforce scope in the backend by resolving partition membership at request time

The inventory handler will resolve the effective partition set before building the node map. In scoped mode it will keep only the configured partition, return only nodes that belong to it, and reject a conflicting `?partition=` query. The switch admission path will also enforce the same scope:

- `slurm_to_openstack` uses the configured cloud partition as the effective partition when the request omits `slurm_partition`.
- `slurm_to_openstack` rejects any explicit `slurm_partition` that does not match the configured cloud partition.
- `openstack_to_slurm` verifies that the requested `node_name` belongs to the configured cloud partition before creating an execution.

This makes the restriction durable even if a caller bypasses the dashboard or uses stale frontend code.

**Alternatives considered**

- Filter only in the dashboard: rejected because it would be trivial to bypass and would not satisfy the "only this partition can switch" requirement.
- Validate only `slurm_to_openstack`: rejected because `openstack_to_slurm` can still target a concrete node and therefore also needs scope enforcement.

### Decision: Reuse the existing runtime config whitelist and switch the dashboard into fixed-partition mode

The nginx runtime config generator will whitelist one additional safe field, `slurmCloudPartition`. When that field is non-empty, the dashboard will initialize into fixed-partition mode: it will request inventory for that partition, omit the "Show all partitions" option, render only the configured partition in the list, and always send that partition in dashboard-triggered `slurm_to_openstack` requests.

This keeps runtime config synchronous and startup-injected, matching the existing SIF-path hint pattern.

**Alternatives considered**

- Add a new authenticated API for cloud partition settings: rejected because the runtime config mechanism already exists for safe dashboard-only settings.
- Keep the "All partitions" view and merely preselect the configured partition: rejected because it still advertises a broader operating scope than the deployment intends.

### Decision: Keep the API surface stable and document scoped semantics through Swagger and operator docs

The change will not add new REST routes or payload fields. Instead, Swagger annotations and docs will describe the conditional semantics for `GET /v1/dashboard/inventory` and `POST /v1/switches` when `SLURM_CLOUD_PARTITION` is configured.

This minimizes compatibility churn while still making the new behavior explicit to operators and API users.

**Alternatives considered**

- Introduce separate scoped endpoints: rejected because the existing routes already model the required operations and only need stronger admission rules.

## Risks / Trade-offs

- [Risk] nginx and daemon may observe different `SLURM_CLOUD_PARTITION` values. → Mitigation: wire both services from the same `.env` source and document that both containers must be restarted together after changes.
- [Risk] Request-time partition membership checks add extra Slurm reads on switch creation. → Mitigation: keep the first version simple and synchronous; switch creation volume is low enough that correctness is more important than caching.
- [Risk] A misconfigured partition name could make the dashboard appear empty or cause request failures. → Mitigation: fail with explicit inventory/switch errors and document that the configured value must match a real Slurm partition name.
- [Risk] Older dashboard assets might still omit `slurm_partition` in scoped mode. → Mitigation: backend defaults missing `slurm_partition` to the configured cloud partition instead of requiring a new client immediately.

## Migration Plan

1. Add `SLURM_CLOUD_PARTITION` to daemon config loading, deployment env wiring, and nginx runtime config generation.
2. Update inventory filtering and switch admission rules to enforce the configured partition scope.
3. Update dashboard JavaScript to honor `slurmCloudPartition` and render fixed-partition behavior.
4. Refresh tests, operator docs, and Swagger artifacts to describe the optional scoped mode.

Rollback is straightforward: unset `SLURM_CLOUD_PARTITION` to restore current behavior, or revert the change if the deployment should not expose scoped mode at all.

## Open Questions

None.
