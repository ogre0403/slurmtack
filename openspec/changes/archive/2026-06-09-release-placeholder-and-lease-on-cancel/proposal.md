## Why

`slurm_to_openstack` execution 在 placeholder job 已經啟動、且 node lease 已經寫入 datastore 的等待邊界上接受 cancel 時，系統目前可能只把 execution 結束，卻留下仍在 running 的 placeholder job 和仍被標註佔用中的 lease record。這會讓同一台 node 無法再次切換，還需要人工取消 Slurm job 和手動修 datastore。

## What Changes

- Strengthen cancellation cleanup so a cancelled `slurm_to_openstack` execution always tears down any placeholder job already associated with the execution before reaching `cancelled`.
- Require cancellation cleanup to release any lease record currently held by the cancelling execution before finalizing terminal cancellation.
- Clarify that cancellation cleanup must inspect the execution's currently claimed resources, not only the wait state that originally accepted the cancellation request.
- Add regression coverage for late-allocation and wait-boundary cancellation cases where placeholder job and lease cleanup previously diverged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `switch-cancellation`: cancellation cleanup must remove placeholder-job and lease artifacts that are already attached to the execution, even when the cancellation was claimed from an earlier wait state.

## Impact

- Affected code: `internal/orchestrator`, `internal/service`, `internal/mq`, and related store-backed cancellation tests.
- Affected systems: Slurm placeholder job lifecycle, execution cancellation flow, and SQLite lease persistence.
- Operator impact: cancelling a stuck switch no longer requires manual Slurm job cleanup or direct datastore edits before retrying the node switch.
