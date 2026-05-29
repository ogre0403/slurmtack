## 1. Request-Time Ownership Guard

- [x] 1.1 Extend `internal/service.SwitchService` with access to Slurm node-state lookup for `openstack_to_slurm` admission, while preserving current behavior when no Slurm-backed guard is configured.
- [x] 1.2 Reuse the existing Slurm attach-state classifier to reject `openstack_to_slurm` requests whose target node is already in an active schedulable Slurm state before execution persistence.
- [x] 1.3 Return clear wrapped request errors for both "already under Slurm ownership" and request-time Slurm lookup failures, and ensure rejected requests do not publish MQ events.

## 2. API and Wiring Changes

- [x] 2.1 Wire the runtime Slurm client into the switch service in `cmd/main.go` so API-created `openstack_to_slurm` requests can perform the ownership guard in production.
- [x] 2.2 Keep `internal/api` mapping the duplicate-ownership rejection to a client-visible 4xx response and preserve existing successful request behavior for valid `openstack_to_slurm` and `slurm_to_openstack` submissions.

## 3. Verification

- [x] 3.1 Add service tests covering active Slurm states that must reject, resumable non-active Slurm states that must still accept, and Slurm lookup failure behavior.
- [x] 3.2 Add API tests proving a duplicate `openstack_to_slurm` request returns a client-visible rejection and does not create a persisted execution.
- [x] 3.3 Run the relevant OpenSpec validation and targeted Go tests for the touched API, service, and Slurm packages.
