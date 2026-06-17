## Why

`slurm_to_openstack` currently treats placeholder allocation as a pure MQ wait: once the placeholder job is submitted and an execution enters `awaiting_source_allocation`, the daemon stops progressing until an allocation event arrives. If the placeholder job fails before the agent can publish that event, the execution stays `active` indefinitely and the dashboard keeps showing a waiting state instead of the real failure.

The repo already documents that immediate placeholder-job failures such as a missing SIF should be detected by the daemon, but the current runtime path does not satisfy that contract. This gap now blocks operators because they must inspect Slurm manually and infer that the switch will never recover on its own.

## What Changes

- Add daemon-side detection for placeholder jobs that reach a terminal Slurm state before any allocation event is received.
- Extend the Slurm client contract so the daemon can query placeholder job state during `awaiting_source_allocation`.
- Record an operator-visible failure reason on the waiting step and execution when allocation fails before node binding.
- Refresh the dashboard's selected execution detail so a failure transition during the wait is reflected without requiring the operator to reselect the execution.
- Add regression coverage for placeholder allocation waits that end in Slurm-side job failure instead of MQ success.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `gpu-node-switch-allocation`: change placeholder-allocation waiting so `slurm_to_openstack` fails when the placeholder job reaches a terminal non-allocation outcome before binding a node.
- `slurmrestd-client`: add placeholder job state lookup behavior needed by the daemon while waiting for allocation.
- `gpu-node-switch-observability`: persist a readable operator-visible failure summary for allocation-wait failures, not only precheck failures.
- `node-switch-dashboard`: keep the selected execution detail in sync with failed allocation waits so the UI stops presenting stale waiting information.

## Impact

- Affected code: `internal/orchestrator`, `internal/slurm`, execution/step failure recording paths, and `docker/nginx/html/dashboard.js`.
- Affected systems: Slurm placeholder lifecycle, daemon async wait handling, and dashboard execution drilldown.
- APIs: no external REST shape change is required; the change relies on existing execution and step-detail responses plus Slurm job-state reads.
