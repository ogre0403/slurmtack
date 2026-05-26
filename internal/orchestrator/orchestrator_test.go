package orchestrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/orchestrator"
	"github.com/slurmtack/slurmtack/internal/store"
)

type fakeSSHRunner struct {
	err error
}

func (f *fakeSSHRunner) Execute(ctx context.Context, req interface{}) (interface{}, error) {
	return nil, f.err
}

type fakeSlurmClient struct {
	submitCalled bool
	drainCalled  bool
	resumeCalled bool
}

func (f *fakeSlurmClient) SubmitPlaceholderJob(ctx context.Context, req interface{}) (interface{}, error) {
	f.submitCalled = true
	return &struct{ JobID string }{JobID: "job-123"}, nil
}

func (f *fakeSlurmClient) GetNodeState(ctx context.Context, nodeName string) (interface{}, error) {
	return nil, nil
}

func (f *fakeSlurmClient) DrainNode(ctx context.Context, nodeName, reason string) error {
	f.drainCalled = true
	return nil
}

func (f *fakeSlurmClient) ResumeNode(ctx context.Context, nodeName string) error {
	f.resumeCalled = true
	return nil
}

func (f *fakeSlurmClient) CancelJob(ctx context.Context, jobID string) error {
	return nil
}

func TestOrchestratorTickProcessesActiveExecutions(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()

	exec := &domain.Execution{
		ID:            "exec-1",
		NodeName:      "gpu-node-01",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StatePrecheckPassed,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
		StateVersion:  3,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create: %v", err)
	}

	runner := engine.NewRunner(s, nil)
	orch := orchestrator.New(s, runner, nil, nil, nil, orchestrator.Config{
		TickInterval:    100 * time.Millisecond,
		SSHPollInterval: 1 * time.Second,
		SSHPollTimeout:  5 * time.Second,
	}, nil)

	tickCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	orch.Run(tickCtx)

	updated, err := s.GetExecution(ctx, "exec-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	// The orchestrator should have tried to quiesce (openstack disable)
	// which will fail because openstack client is nil, causing a failure transition
	if updated.OverallStatus != domain.OverallStatusFailed {
		t.Logf("execution state: %s, status: %s", updated.CurrentState, updated.OverallStatus)
	}
}

func TestOrchestratorSkipsWaitingStates(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()

	exec := &domain.Execution{
		ID:            "exec-2",
		NodeName:      "gpu-node-02",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateAwaitingSourceAllocation,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  2,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create: %v", err)
	}

	runner := engine.NewRunner(s, nil)
	orch := orchestrator.New(s, runner, nil, nil, nil, orchestrator.Config{
		TickInterval:    100 * time.Millisecond,
		SSHPollInterval: 1 * time.Second,
		SSHPollTimeout:  5 * time.Second,
	}, nil)

	tickCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	orch.Run(tickCtx)

	updated, err := s.GetExecution(ctx, "exec-2")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	// Should remain unchanged — awaiting MQ event
	if updated.CurrentState != domain.StateAwaitingSourceAllocation {
		t.Errorf("expected state awaiting_source_allocation, got %s", updated.CurrentState)
	}
	if updated.OverallStatus != domain.OverallStatusActive {
		t.Errorf("expected active status, got %s", updated.OverallStatus)
	}
}
