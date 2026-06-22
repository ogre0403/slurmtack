package remote

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/slurmtack/slurmtack/internal/trace"
)

type SSHExecutorConfig struct {
	User         string
	Port         string
	Options      []string
	IdentityFile string
}

type ExecSSHExecutor struct {
	config SSHExecutorConfig
	logger *slog.Logger
}

type sshInvocation struct {
	target        string
	remoteCommand string
	args          []string
}

type scpInvocation struct {
	target     string
	remoteSpec string
	args       []string
}

func NewExecSSHExecutor(cfg SSHExecutorConfig, logger *slog.Logger) *ExecSSHExecutor {
	return &ExecSSHExecutor{config: cfg, logger: trace.OrDefault(logger)}
}

func (e *ExecSSHExecutor) Run(ctx context.Context, req CommandRequest) (stdout, stderr string, exitCode int, err error) {
	runCtx := ctx
	var cancel context.CancelFunc
	if req.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	invocation := e.renderInvocation(req)
	e.logDispatch(req, invocation)

	cmd := exec.CommandContext(runCtx, "ssh", invocation.args...)
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if err == nil {
		return stdout, stderr, 0, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return stdout, stderr, exitErr.ExitCode(), nil
	}

	return stdout, stderr, -1, fmt.Errorf("ssh command failed: %w", err)
}

// Copy stages a local file onto the target node with scp, reusing the SSH user,
// port, identity, and options configured for command execution. scp takes the
// port via -P (uppercase) rather than ssh's -p.
func (e *ExecSSHExecutor) Copy(ctx context.Context, req StageRequest) error {
	runCtx := ctx
	var cancel context.CancelFunc
	if req.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	invocation := e.renderCopyInvocation(req)
	e.logCopyDispatch(req, invocation)

	cmd := exec.CommandContext(runCtx, "scp", invocation.args...)
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		stderr := strings.TrimSpace(stderrBuf.String())
		if stderr != "" {
			return fmt.Errorf("scp command failed: %w: %s", err, stderr)
		}
		return fmt.Errorf("scp command failed: %w", err)
	}
	return nil
}

func (e *ExecSSHExecutor) renderCopyInvocation(req StageRequest) scpInvocation {
	scpArgs := []string{"-o", "BatchMode=yes"}
	if e.config.Port != "" {
		scpArgs = append(scpArgs, "-P", e.config.Port)
	}
	if e.config.IdentityFile != "" {
		scpArgs = append(scpArgs, "-i", e.config.IdentityFile)
	}
	for _, opt := range e.config.Options {
		if opt == "" {
			continue
		}
		scpArgs = append(scpArgs, "-o", opt)
	}

	target := req.Host
	if e.config.User != "" {
		target = e.config.User + "@" + req.Host
	}
	remoteSpec := target + ":" + req.RemotePath

	scpArgs = append(scpArgs, req.LocalPath, remoteSpec)

	return scpInvocation{
		target:     target,
		remoteSpec: remoteSpec,
		args:       scpArgs,
	}
}

func (e *ExecSSHExecutor) logCopyDispatch(req StageRequest, invocation scpInvocation) {
	attrs := []any{
		"component", "remote",
		"target", invocation.target,
		"local_path", req.LocalPath,
		"remote_path", req.RemotePath,
	}
	if req.ExecutionID != "" {
		attrs = append(attrs, "execution_id", req.ExecutionID)
	}
	if req.StepName != "" {
		attrs = append(attrs, "step_name", req.StepName)
	}

	e.logger.Info(trace.EventSCPDispatch, attrs...)
}

func (e *ExecSSHExecutor) renderInvocation(req CommandRequest) sshInvocation {
	sshArgs := []string{"-o", "BatchMode=yes"}
	if e.config.Port != "" {
		sshArgs = append(sshArgs, "-p", e.config.Port)
	}
	if e.config.IdentityFile != "" {
		sshArgs = append(sshArgs, "-i", e.config.IdentityFile)
	}
	for _, opt := range e.config.Options {
		if opt == "" {
			continue
		}
		sshArgs = append(sshArgs, "-o", opt)
	}

	target := req.Host
	if e.config.User != "" {
		target = e.config.User + "@" + req.Host
	}

	remoteCommand := req.Command
	if len(req.Args) > 0 {
		remoteCommand += " " + shellQuoteArgs(req.Args)
	}

	sshArgs = append(sshArgs, target, remoteCommand)

	return sshInvocation{
		target:        target,
		remoteCommand: remoteCommand,
		args:          sshArgs,
	}
}

func (e *ExecSSHExecutor) logDispatch(req CommandRequest, invocation sshInvocation) {
	attrs := []any{
		"component", "remote",
		"target", invocation.target,
		"remote_command", invocation.remoteCommand,
	}
	if req.ExecutionID != "" {
		attrs = append(attrs, "execution_id", req.ExecutionID)
	}
	if req.StepName != "" {
		attrs = append(attrs, "step_name", req.StepName)
	}

	e.logger.Info(trace.EventSSHDispatch, attrs...)
}

func shellQuoteArgs(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
