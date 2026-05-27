package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type CommandResult struct {
	ExitCode int             `json:"exit_code"`
	Stdout   string          `json:"stdout"`
	Stderr   string          `json:"stderr"`
	Duration time.Duration   `json:"duration"`
	Data     json.RawMessage `json:"data,omitempty"`
}

type CommandRequest struct {
	Host        string
	Command     string
	Args        []string
	ExecutionID string
	StepName    string
	Timeout     time.Duration
}

type Runner interface {
	Execute(ctx context.Context, req CommandRequest) (*CommandResult, error)
}

type SSHRunner struct {
	executor SSHExecutor
}

type SSHExecutor interface {
	Run(ctx context.Context, req CommandRequest) (stdout, stderr string, exitCode int, err error)
}

func NewSSHRunner(executor SSHExecutor) *SSHRunner {
	return &SSHRunner{executor: executor}
}

func (r *SSHRunner) Execute(ctx context.Context, req CommandRequest) (*CommandResult, error) {
	start := time.Now()
	stdout, stderr, exitCode, err := r.executor.Run(ctx, req)
	duration := time.Since(start)

	if err != nil {
		return nil, fmt.Errorf("ssh execution failed on %s: %w", req.Host, err)
	}

	result := &CommandResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: duration,
	}

	if exitCode == 0 && stdout != "" {
		var data json.RawMessage
		if json.Unmarshal([]byte(stdout), &data) == nil {
			result.Data = data
		}
	}

	return result, nil
}
