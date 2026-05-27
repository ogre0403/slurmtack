## 1. SSH config model

- [x] 1.1 Extend `internal/config.Config` and `Load()` to read `SSH_PRIVATE_KEY_PATH` and reject partial SSH runner configuration when the key path is missing or unreadable.
- [x] 1.2 Add focused config tests that cover valid SSH runner setup and startup failures for incomplete SSH transport settings.

## 2. SSH transport and wiring

- [x] 2.1 Extend `internal/remote.SSHExecutorConfig` and `ExecSSHExecutor.Run()` to pass the configured private key file to the local `ssh` command.
- [x] 2.2 Update daemon startup in `cmd/main.go` to construct `ExecSSHExecutor` and `SSHRunner` from validated config and pass the runner into the orchestrator.
- [x] 2.3 Add focused tests for SSH command assembly and daemon wiring so reboot and reachability actions use the configured transport.

## 3. Operator-facing configuration

- [x] 3.1 Update `.env.example` and `docker/.env.example` to document `SSH_PRIVATE_KEY_PATH` alongside the existing SSH runner settings.
- [x] 3.2 Update deployment guidance in `README.md` or related docs to describe how the daemon receives the SSH key path and what validation errors operators should expect.

## 4. Validation

- [x] 4.1 Run focused Go tests for the touched SSH config, remote executor, and daemon startup paths.