package orchestrator

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/store"
)

type recordingSSHRunner struct {
	requests []remote.CommandRequest
	result   *remote.CommandResult
	err      error
}

func (r *recordingSSHRunner) Execute(_ context.Context, req remote.CommandRequest) (*remote.CommandResult, error) {
	r.requests = append(r.requests, req)
	if r.result != nil {
		return r.result, r.err
	}
	return &remote.CommandResult{ExitCode: 0}, r.err
}

func newExecutionForState(state domain.SwitchState) *domain.Execution {
	return &domain.Execution{
		ID:            "ssh-exec-1",
		NodeName:      "gpu-node-01",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  state,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
		StateVersion:  7,
		OverallStatus: domain.OverallStatusActive,
	}
}

func newSSHOrchestrator(t *testing.T, sshRunner remote.Runner, logger *slog.Logger) (*Orchestrator, store.Store, *domain.Execution) {
	t.Helper()

	s := store.NewMemoryStore()
	exec := newExecutionForState(domain.StateHostReconfiguring)
	ctx := context.Background()
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	runner := engine.NewRunner(s, logger)
	orch := New(s, runner, sshRunner, nil, nil, Config{
		SSHPollInterval: time.Millisecond,
		SSHPollTimeout:  50 * time.Millisecond,
	}, logger)

	return orch, s, exec
}

func TestDoRebootDispatchesExactCommand(t *testing.T) {
	logger, _ := newCaptureLogger()
	sshRunner := &recordingSSHRunner{}
	orch, s, exec := newSSHOrchestrator(t, sshRunner, logger)

	if err := orch.doReboot(context.Background(), exec); err != nil {
		t.Fatalf("doReboot() error = %v", err)
	}
	if len(sshRunner.requests) != 1 {
		t.Fatalf("sshRunner requests = %d, want 1", len(sshRunner.requests))
	}
	req := sshRunner.requests[0]
	if req.Host != exec.NodeName {
		t.Fatalf("reboot host = %q, want %q", req.Host, exec.NodeName)
	}
	if req.Command != "reboot" {
		t.Fatalf("reboot command = %q, want %q", req.Command, "reboot")
	}
	if len(req.Args) != 0 {
		t.Fatalf("reboot args = %#v, want none", req.Args)
	}
	if req.ExecutionID != exec.ID {
		t.Fatalf("reboot execution_id = %q, want %q", req.ExecutionID, exec.ID)
	}
	if req.StepName != "reboot" {
		t.Fatalf("reboot step_name = %q, want %q", req.StepName, "reboot")
	}

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateRebooting {
		t.Fatalf("execution state = %s, want %s", updated.CurrentState, domain.StateRebooting)
	}
}

func TestDoSSHPollThreadsExecutionMetadataIntoProbe(t *testing.T) {
	logger, _ := newCaptureLogger()
	sshRunner := &recordingSSHRunner{}
	orch, s, exec := newSSHOrchestrator(t, sshRunner, logger)
	exec.CurrentState = domain.StateRebooting
	if err := s.UpdateExecution(context.Background(), exec); err != nil {
		t.Fatalf("update execution: %v", err)
	}

	if err := orch.doSSHPoll(context.Background(), exec); err != nil {
		t.Fatalf("doSSHPoll() error = %v", err)
	}
	if len(sshRunner.requests) != 1 {
		t.Fatalf("sshRunner requests = %d, want 1", len(sshRunner.requests))
	}
	req := sshRunner.requests[0]
	if req.Host != exec.NodeName {
		t.Fatalf("probe host = %q, want %q", req.Host, exec.NodeName)
	}
	if req.Command != "hostname" {
		t.Fatalf("probe command = %q, want %q", req.Command, "hostname")
	}
	if len(req.Args) != 0 {
		t.Fatalf("probe args = %#v, want none", req.Args)
	}
	if req.ExecutionID != exec.ID {
		t.Fatalf("probe execution_id = %q, want %q", req.ExecutionID, exec.ID)
	}
	if req.StepName != sshProbeStepName {
		t.Fatalf("probe step_name = %q, want %q", req.StepName, sshProbeStepName)
	}

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateHostReachable {
		t.Fatalf("execution state = %s, want %s", updated.CurrentState, domain.StateHostReachable)
	}
}
