## Why

`openstack_to_slurm` 目前會先進入 workflow，等到 `verify_source_quiesce` 才發現 source node 上仍有 VM 或 in-flight migration，導致 execution 卡在過晚的階段才暴露「其實不能切換」。這讓 operator 看起來像流程停住，而不是在 precheck 階段被明確拒絕，也缺少可以直接顯示在 execution detail steps 的拒絕原因。

## What Changes

- Tighten the `openstack_to_slurm` precheck so it verifies source readiness before quiesce, including resident instances, active migrations, and the required OpenStack compute-service state.
- Fail the execution as `precheck_blocked` during precheck when the node cannot be switched yet, instead of first surfacing those blockers from `verify_source_quiesce`.
- Persist operator-visible blocker summaries for failed precheck steps so the rejection reason is available from the durable step timeline.
- Expose the step-level rejection reason through the execution step API and render it in the dashboard execution detail steps view.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `orchestrator`: change `openstack_to_slurm` readiness gating so resident VMs and active migrations are rejected during precheck instead of first being surfaced by source-quiesce verification.
- `gpu-node-switch-observability`: persist step-level blocker summaries for failed `precheck_blocked` executions so operators can inspect why switching was refused.
- `rest-api`: extend the execution step timeline contract so failed steps can return an operator-visible rejection reason in addition to failure class and evidence paths.
- `node-switch-dashboard`: show precheck rejection reasons inside execution detail step rendering.

## Impact

- Affected code: `internal/orchestrator`, `internal/openstack`, step persistence/store code, API DTO/handler code, Swagger docs, and `docker/nginx/html/dashboard.js`.
- Affected APIs: `GET /v1/switches/:id/steps` response shape and its dashboard consumer.
- Data/storage impact: likely a step-history schema change to persist a short rejection summary/detail alongside existing step metadata.
- Operational impact: `openstack_to_slurm` requests with resident VMs or active migrations will fail earlier and explain the refusal directly in execution detail steps.
