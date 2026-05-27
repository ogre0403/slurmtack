package remote

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type SSHExecutorConfig struct {
	User         string
	Port         string
	Options      []string
	IdentityFile string
}

type ExecSSHExecutor struct {
	config SSHExecutorConfig
}

func NewExecSSHExecutor(cfg SSHExecutorConfig) *ExecSSHExecutor {
	return &ExecSSHExecutor{config: cfg}
}

func (e *ExecSSHExecutor) Run(ctx context.Context, host string, command string, args []string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	runCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(runCtx, "ssh", e.buildSSHArgs(host, command, args)...)
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

func (e *ExecSSHExecutor) buildSSHArgs(host string, command string, args []string) []string {

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

	target := host
	if e.config.User != "" {
		target = e.config.User + "@" + host
	}

	remoteCommand := command
	if len(args) > 0 {
		remoteCommand += " " + shellQuoteArgs(args)
	}

	sshArgs = append(sshArgs, target, remoteCommand)

	return sshArgs
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
