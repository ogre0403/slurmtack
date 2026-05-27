## Why

The live `openstack_to_slurm` flow can enter `source_quiescing` and stop there even when connectivity and external services are healthy. The current orchestrator disables the OpenStack compute service and transitions into `source_quiescing`, but it does not implement the documented O2S quiesce-verification step that should advance the execution to `source_detached` once the source side is actually drained.

## What Changes

- Update the orchestrator's `openstack_to_slurm` state-to-action mapping so `source_quiescing` is an active verification state instead of a permanent wait state.
- Require the O2S quiesce flow to verify that the OpenStack compute service is disabled and that the host has no resident instances or active migrations before transitioning to `source_detached`.
- Define how the orchestrator behaves when quiesce verification is still in progress versus when it definitively fails, including trace coverage and focused tests.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `orchestrator`: `openstack_to_slurm` executions in `source_quiescing` must be re-evaluated by the tick loop and automatically advanced to `source_detached` after source quiesce verification succeeds.

## Impact

- Affected code: `internal/orchestrator`, related orchestrator tests, and any helper logic used to inspect OpenStack quiesce state.
- External systems: OpenStack Nova service status, instance listing, and migration listing become part of the live O2S progression check.
- APIs: no REST or MQ contract changes are expected.