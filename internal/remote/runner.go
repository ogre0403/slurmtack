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

// StageRequest describes a local file to copy to a remote node over scp,
// reusing the same SSH transport configuration as command execution.
type StageRequest struct {
	Host        string
	LocalPath   string
	RemotePath  string
	ExecutionID string
	StepName    string
	Timeout     time.Duration
}

type Runner interface {
	Execute(ctx context.Context, req CommandRequest) (*CommandResult, error)
	Stage(ctx context.Context, req StageRequest) error
}

type SSHRunner struct {
	executor SSHExecutor
}

type SSHExecutor interface {
	Run(ctx context.Context, req CommandRequest) (stdout, stderr string, exitCode int, err error)
	Copy(ctx context.Context, req StageRequest) error
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

// Stage copies a local file to the target node over scp, surfacing transport
// failures so the caller can abort before depending on the staged artifact.
func (r *SSHRunner) Stage(ctx context.Context, req StageRequest) error {
	if err := r.executor.Copy(ctx, req); err != nil {
		return fmt.Errorf("scp staging failed on %s: %w", req.Host, err)
	}
	return nil
}
