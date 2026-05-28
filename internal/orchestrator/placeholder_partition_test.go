package orchestrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/orchestrator"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

type capturePlaceholderSlurmClient struct {
	submitRequests []slurm.PlaceholderJobRequest
}

func (f *capturePlaceholderSlurmClient) SubmitPlaceholderJob(_ context.Context, req slurm.PlaceholderJobRequest) (*slurm.PlaceholderJobResult, error) {
	f.submitRequests = append(f.submitRequests, req)
	return &slurm.PlaceholderJobResult{JobID: "job-123"}, nil
}

func (f *capturePlaceholderSlurmClient) GetNodeState(_ context.Context, nodeName string) (*slurm.NodeState, error) {
	return nil, nil
}

func (f *capturePlaceholderSlurmClient) DrainNode(_ context.Context, nodeName, reason string) error {
	return nil
}

func (f *capturePlaceholderSlurmClient) ResumeNode(_ context.Context, nodeName string) error {
	return nil
}

func (f *capturePlaceholderSlurmClient) CancelJob(_ context.Context, jobID string) error {
	return nil
}

func TestOrchestratorSubmitPlaceholderUsesRequestedPartition(t *testing.T) {
	fakeSlurm := &capturePlaceholderSlurmClient{}
	exec := &domain.Execution{
		ID:                       "exec-partition",
		Direction:                domain.DirectionSlurmToOpenStack,
		RequestedBy:              "operator",
		RequestedAt:              time.Now(),
		CurrentState:             domain.StateRequested,
		DesiredOwner:             domain.OwnerOpenStack,
		PreviousOwner:            domain.OwnerSlurm,
		OverallStatus:            domain.OverallStatusActive,
		RequestedSlurmConstraint: "gpu-a100",
		RequestedSlurmPartition:  "gpu-maint",
	}

	runOrchestratorPlaceholderSubmission(t, exec, fakeSlurm)

	if len(fakeSlurm.submitRequests) != 1 {
		t.Fatalf("submitRequests = %d, want 1", len(fakeSlurm.submitRequests))
	}
	if fakeSlurm.submitRequests[0].Partition != "gpu-maint" {
		t.Fatalf("Partition = %q, want gpu-maint", fakeSlurm.submitRequests[0].Partition)
	}
}

func TestOrchestratorSubmitPlaceholderLeavesPartitionEmptyWhenOmitted(t *testing.T) {
	fakeSlurm := &capturePlaceholderSlurmClient{}
	exec := &domain.Execution{
		ID:                       "exec-no-partition",
		Direction:                domain.DirectionSlurmToOpenStack,
		RequestedBy:              "operator",
		RequestedAt:              time.Now(),
		CurrentState:             domain.StateRequested,
		DesiredOwner:             domain.OwnerOpenStack,
		PreviousOwner:            domain.OwnerSlurm,
		OverallStatus:            domain.OverallStatusActive,
		RequestedSlurmConstraint: "gpu-a100",
	}

	runOrchestratorPlaceholderSubmission(t, exec, fakeSlurm)

	if len(fakeSlurm.submitRequests) != 1 {
		t.Fatalf("submitRequests = %d, want 1", len(fakeSlurm.submitRequests))
	}
	if fakeSlurm.submitRequests[0].Partition != "" {
		t.Fatalf("Partition = %q, want empty string", fakeSlurm.submitRequests[0].Partition)
	}
}

func TestOrchestratorRunDoesNotAutoDiscoverRequestedExecutions(t *testing.T) {
	fakeSlurm := &capturePlaceholderSlurmClient{}
	exec := &domain.Execution{
		ID:            "exec-no-auto-discovery",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "operator",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateRequested,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		OverallStatus: domain.OverallStatusActive,
	}

	s := store.NewMemoryStore()
	ctx := context.Background()
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution() error = %v", err)
	}

	runner := engine.NewRunner(s, nil)
	orch := orchestrator.New(s, runner, nil, fakeSlurm, nil, orchestrator.Config{
		TickInterval:    10 * time.Millisecond,
		SSHPollInterval: time.Second,
		SSHPollTimeout:  5 * time.Second,
	}, nil)

	runCtx, cancel := context.WithTimeout(ctx, 60*time.Millisecond)
	defer cancel()
	orch.Run(runCtx)

	updated, err := s.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if updated.CurrentState != domain.StateRequested {
		t.Fatalf("CurrentState = %s, want %s", updated.CurrentState, domain.StateRequested)
	}
	if len(fakeSlurm.submitRequests) != 0 {
		t.Fatalf("submitRequests = %d, want 0 without explicit admission", len(fakeSlurm.submitRequests))
	}
}

func runOrchestratorPlaceholderSubmission(t *testing.T, exec *domain.Execution, fakeSlurm *capturePlaceholderSlurmClient) {
	t.Helper()

	s := store.NewMemoryStore()
	ctx := context.Background()
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution() error = %v", err)
	}

	runner := engine.NewRunner(s, nil)
	orch := orchestrator.New(s, runner, nil, fakeSlurm, nil, orchestrator.Config{
		TickInterval:    10 * time.Millisecond,
		SSHPollInterval: time.Second,
		SSHPollTimeout:  5 * time.Second,
	}, nil)

	tickCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	orch.AdmitExecution(tickCtx, exec.ID)

	deadline := time.Now().Add(150 * time.Millisecond)
	for time.Now().Before(deadline) {
		updated, err := s.GetExecution(ctx, exec.ID)
		if err != nil {
			t.Fatalf("GetExecution() error = %v", err)
		}
		if updated.CurrentState == domain.StateAwaitingSourceAllocation {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	updated, err := s.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if updated.CurrentState != domain.StateAwaitingSourceAllocation {
		t.Fatalf("CurrentState = %s, want %s", updated.CurrentState, domain.StateAwaitingSourceAllocation)
	}
}
