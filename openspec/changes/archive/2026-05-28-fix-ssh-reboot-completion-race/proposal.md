## Why

The orchestrator currently treats the first successful `hostname` probe after issuing `reboot` as proof that the reboot finished. On real hosts, SSH can still succeed briefly while the old OS session is shutting down, which lets the workflow advance even though the reboot has not actually started.

This race makes the post-reboot gate unreliable and can trigger downstream steps against a host that is still on the old boot. The behavior needs a stronger reboot completion contract before implementation work proceeds further.

## What Changes

- Tighten the post-reboot SSH waiting behavior so a host is only considered back after there is evidence that the original SSH-reachable session went away and a later probe succeeds.
- Define timeout and failure behavior for hosts that never become unreachable after `reboot` or never return after becoming unreachable.
- Add regression coverage for the race where `reboot` is dispatched and an immediate `hostname` probe still succeeds against the pre-reboot OS.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `ssh-reachability`: Change reboot completion semantics so a single successful `hostname` probe immediately after `reboot` is not enough to declare the host rebooted and reachable.

## Impact

- Affected code is centered in `internal/orchestrator/reachability.go`, `internal/orchestrator/orchestrator.go`, and their reboot/reachability tests.
- The change is expected to alter internal reboot verification logic and trace coverage, but it does not require an external API or persistence schema change.
- The result should prevent false-positive reboot completion and reduce unsafe progression into later orchestration steps.