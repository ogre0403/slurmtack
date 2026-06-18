## 1. Boot-completion SSH probe

- [x] 1.1 Change the post-reboot probe in `internal/orchestrator/reachability.go` from `hostname` to a boot-completion check (`test ! -f /run/nologin`)
- [x] 1.2 Extend `classifyProbeResult` to recognize "connected but still booting" (`pam_nologin` / "System is booting up" message, or non-zero `test` exit) as a not-ready result, not a hard error
- [x] 1.3 Ensure the "still booting" result folds into the existing reboot-observed / keep-polling branch and does NOT satisfy reachability
- [x] 1.4 Ignore the post-quantum key-exchange stderr banner when the boot-completion check exits 0
- [x] 1.5 Update trace/log fields and the `sshProbeStepName` usage as needed for the new probe semantics

## 2. Bounded retry on slurmd restore

- [x] 2.1 Add a boot-transient classifier helper (matches `pam_nologin` / "System is booting up" / session-closed-during-login) usable by the orchestrator
- [x] 2.2 Wrap the OpenStack-to-Slurm enable/start path so `runSlurmdServiceCommand` retries boot-transient failures with bounded attempts and short backoff (bounded under `slurmdCommandTimeout`)
- [x] 2.3 Ensure non-transient failures still fail immediately and that exhausted retries fail the step without issuing `ResumeNode`
- [x] 2.4 Confirm the `slurm_to_openstack` stop/disable path is unaffected (no retry added there)

## 3. Tests

- [x] 3.1 Update `internal/orchestrator/reachability_test.go` to expect the new probe command and assert the "connected but still booting" keep-polling behavior
- [x] 3.2 Add a test where the probe succeeds only after the node leaves the `pam_nologin` window
- [x] 3.3 Update `internal/orchestrator/attach_guard_test.go` for any changed slurmd command expectations and add a boot-transient-retry-then-success case
- [x] 3.4 Add a test asserting exhausted boot-transient retries fail the attach step terminally and do not call `ResumeNode`
- [x] 3.5 Run `go test ./...` and `go vet ./...`; ensure the full suite passes

## 4. Validation

- [x] 4.1 Run `openspec validate harden-attach-boot-readiness --strict` and resolve any issues
- [ ] 4.2 Confirm an `openstack_to_slurm` switch on a freshly rebooted node reaches `completed` without the `slurmd start failed` error
