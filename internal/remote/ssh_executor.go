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
