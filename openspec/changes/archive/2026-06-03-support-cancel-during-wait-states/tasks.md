## 1. Extend the execution model for cancellation

- [x] 1.1 Add the new cancellation states and persisted cancellation-context field(s) to the domain model, transition map, and storage schema.
- [x] 1.2 Ensure execution status mapping treats `cancelled` as a terminal failed outcome while leaving `cancelling` recoverable by the orchestrator.

## 2. Implement API-side cancellation claiming

- [x] 2.1 Replace the `/v1/switches/:id/cancel` 501 stub with a service-backed cancel handler that validates the current state and returns 202/404/409 as specified.
- [x] 2.2 Implement idempotent cancellation claiming so eligible wait states move to `cancelling` with the original wait state captured for later cleanup.

## 3. Add orchestrator-owned cancellation cleanup

- [x] 3.1 Extend orchestrator action selection and startup recovery so `cancelling` executions run cancellation cleanup instead of normal workflow progression.
- [x] 3.2 Implement cleanup plans for `awaiting_target_node`, `awaiting_source_allocation`, and `source_quiescing` in both directions, including placeholder job cancellation, Slurm resume, OpenStack compute re-enable, and lease release as applicable.
- [x] 3.3 Terminalize successful cleanup as `cancelled` with `cancelled_by_user`, and route cleanup failures to the non-destructive failure path with a cancellation-specific error code.

## 4. Verify behavior and update docs

- [x] 4.1 Add API, service, orchestrator, MQ-event, and recovery tests covering accepted cancellation states, rejected states, idempotent repeats, and late-event races.
- [x] 4.2 Update README and API/workflow documentation to describe the safe cancellation window, the new `cancelling`/`cancelled` states, and the states that still reject cancel.
