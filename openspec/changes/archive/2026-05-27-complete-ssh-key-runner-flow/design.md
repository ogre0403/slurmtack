## Context

The orchestrator already models reboot and post-reboot host reachability through the `remote.Runner` abstraction, but the daemon startup path currently passes a nil SSH runner into the orchestrator. At the transport layer, `internal/remote/ExecSSHExecutor` shells out to `ssh` with batch mode, port, and free-form `-o` options, but it has no first-class identity file setting. In practice, a deployment that requires key-based login cannot complete the reboot-to-reachability path reliably, and the failure shows up late as `ssh runner not configured` or as opaque authentication problems.

This change should complete the existing design instead of introducing a new transport stack. The repository already exposes SSH-related environment variables, already uses the local `ssh` client, and already has orchestrator logic built around `remote.Runner`.

## Goals / Non-Goals

**Goals:**
- Make the daemon capable of running reboot and reachability SSH commands with an explicitly configured private key file.
- Wire the SSH runner into daemon startup so the existing orchestrator control path can execute its reboot and polling actions.
- Fail early on incomplete SSH runner configuration instead of surfacing misconfiguration only during workflow execution.
- Cover the completed flow with focused tests and deployment-facing configuration updates.

**Non-Goals:**
- Replacing the local `ssh` binary with a native Go SSH client.
- Storing private key contents in environment variables or the database.
- Redesigning host key verification beyond the existing `SSH_OPTIONS` passthrough.
- Adding agent forwarding, bastion host support, or other advanced SSH transport features.

## Decisions

### 1. Represent the SSH identity as a file path

Add `SSH_PRIVATE_KEY_PATH` to daemon configuration and `IdentityFile` to `remote.SSHExecutorConfig`. The executor will translate that value into an explicit `ssh -i <path>` argument.

This keeps key material outside the process environment, matches container and systemd deployment patterns where secrets are mounted as files, and makes the executor command line deterministic enough to test. The main alternative was to keep pushing identity selection into `SSH_OPTIONS` via `IdentityFile=...`, but that leaves the critical authentication path undocumented, hard to validate, and easy to misquote.

### 2. Instantiate the SSH runner during daemon startup

`cmd/main.go` should construct `remote.NewExecSSHExecutor` and `remote.NewSSHRunner` when SSH transport configuration is enabled, then pass that runner into `orchestrator.New`.

`config.Load()` should validate the SSH transport inputs before startup completes. If any SSH transport variable (`SSH_USER`, `SSH_PORT`, `SSH_OPTIONS`, or `SSH_PRIVATE_KEY_PATH`) is set, `SSH_PRIVATE_KEY_PATH` must also be set and point to a readable file. This keeps the daemon from accepting configuration that cannot possibly execute the reboot/reachability path. The rejected alternative is the current lazy failure mode, where the orchestrator hits a nil runner or unusable SSH command only after a workflow reaches reboot.

### 3. Keep polling semantics unchanged and complete only the transport layer

The `remote.Runner` interface, the `hostname` reachability probe, the 5-second per-attempt timeout, and the overall poll interval/timeout contract should stay as they are. The change is about completing the transport and wiring path, not about redesigning the orchestration state machine.

Testing should stay focused on three slices: config validation, SSH command assembly, and daemon wiring. This is lower risk than replacing the transport implementation or changing the orchestrator retry semantics.

## Risks / Trade-offs

- [Mounted key paths vary between local and container deployments] → Document the new variable in both environment examples and deployment guidance so operators provide a valid in-environment path.
- [Host key policy remains operator-managed through `SSH_OPTIONS`] → Keep the current `SSH_OPTIONS` passthrough for this change and defer dedicated known-hosts controls to a follow-up if operations need stricter pinning.
- [Stricter startup validation may reject previously tolerated partial config] → Limit the new validation to SSH transport variables and return a targeted error that names the missing or unreadable key path.

## Migration Plan

- Mount or provision the SSH private key file where the daemon process can read it.
- Set `SSH_PRIVATE_KEY_PATH` and keep or adjust `SSH_USER`, `SSH_PORT`, and `SSH_OPTIONS` for the target environment.
- Deploy the updated daemon so it constructs the SSH runner at startup.
- Roll back by reverting the daemon binary and removing the new key-path configuration; no data migration is required.

## Open Questions

- Should a follow-up change add dedicated variables for known-hosts handling instead of continuing to rely on `SSH_OPTIONS` for that policy?