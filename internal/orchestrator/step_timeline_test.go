package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/openstack"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

type fakeStepSlurmClient struct {
	submitErr error
	drainErr  error
}

func (f *fakeStepSlurmClient) SubmitPlaceholderJob(_ context.Context, _ slurm.PlaceholderJobRequest) (*slurm.PlaceholderJobResult, error) {
	if f.submitErr != nil {
		return nil, f.submitErr
	}
	return &slurm.PlaceholderJobResult{JobID: "job-step-test"}, nil
}

func (f *fakeStepSlurmClient) GetNodeState(_ context.Context, _ string) (*slurm.NodeState, error) {
	return nil, nil
}

func (f *fakeStepSlurmClient) GetNodeStateWithIdentity(_ context.Context, _ string, _ slurm.WorkloadIdentity) (*slurm.NodeState, error) {
	return nil, nil
}

func (f *fakeStepSlurmClient) DrainNode(_ context.Context, _, _ string) error {
	return f.drainErr
}

func (f *fakeStepSlurmClient) ResumeNode(_ context.Context, _ string) error {
	return nil
}

func (f *fakeStepSlurmClient) CancelJob(_ context.Context, _ string) error {
	return nil
}

func (f *fakeStepSlurmClient) CancelJobWithIdentity(_ context.Context, _ string, _ slurm.WorkloadIdentity) error {
	return nil
}

func (f *fakeStepSlurmClient) ListPartitions(_ context.Context) ([]slurm.Partition, error) {
	return nil, nil
}
func (f *fakeStepSlurmClient) GetNodes(_ context.Context) ([]slurm.NodeState, error) {
	return nil, nil
}

func (f *fakeStepSlurmClient) VerifyToken(_ context.Context, _, _ string) error { return nil }

func setupStepOrchestrator(t *testing.T, exec *domain.Execution, slurmClient slurm.Client, osClient *fakeOpenStackClient) (*Orchestrator, store.Store) {
	t.Helper()
	s := store.NewMemoryStore()
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}
	runner := engine.NewRunner(s, nil)
	orch := New(s, runner, nil, slurmClient, osClient, Config{}, nil)
	return orch, s
}

func requireSteps(t *testing.T, s store.Store, executionID string, minCount int) []*domain.StepRecord {
	t.Helper()
	steps, err := s.ListSteps(context.Background(), executionID)
	if err != nil {
		t.Fatalf("ListSteps() error = %v", err)
	}
	if len(steps) < minCount {
		t.Fatalf("expected at least %d steps, got %d", minCount, len(steps))
	}
	return steps
}

func TestStepTimeline_SubmitPlaceholder_PersistsActionAndWaitSteps(t *testing.T) {
	exec := &domain.Execution{
		ID:            "step-submit-1",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateRequested,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		OverallStatus: domain.OverallStatusActive,
	}
	orch, s := setupStepOrchestrator(t, exec, &fakeStepSlurmClient{}, nil)
	orch.processExecution(context.Background(), exec)

	steps := requireSteps(t, s, exec.ID, 2)

	if steps[0].StepName != domain.StepSubmitPlaceholder {
		t.Errorf("step[0].StepName = %q, want %q", steps[0].StepName, domain.StepSubmitPlaceholder)
	}
	if steps[0].Status != domain.StepStatusSucceeded {
		t.Errorf("step[0].Status = %q, want succeeded", steps[0].Status)
	}
	if steps[1].StepName != domain.StepWaitForSourceAllocation {
		t.Errorf("step[1].StepName = %q, want %q", steps[1].StepName, domain.StepWaitForSourceAllocation)
	}
	if steps[1].Status != domain.StepStatusRunning {
		t.Errorf("step[1].Status = %q, want running", steps[1].Status)
	}
}

func TestStepTimeline_QuiesceS2O_PersistsActionAndWaitSteps(t *testing.T) {
	exec := &domain.Execution{
		ID:            "step-quiesce-1",
		NodeName:      "gpu-01",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StatePrecheckPassed,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  3,
		OverallStatus: domain.OverallStatusActive,
	}
	orch, s := setupStepOrchestrator(t, exec, &fakeStepSlurmClient{}, nil)
	orch.processExecution(context.Background(), exec)

	steps := requireSteps(t, s, exec.ID, 2)

	if steps[0].StepName != domain.StepQuiesceSource {
		t.Errorf("step[0].StepName = %q, want %q", steps[0].StepName, domain.StepQuiesceSource)
	}
	if steps[0].Status != domain.StepStatusSucceeded {
		t.Errorf("step[0].Status = %q, want succeeded", steps[0].Status)
	}
	if steps[1].StepName != domain.StepWaitForSourceDrain {
		t.Errorf("step[1].StepName = %q, want %q", steps[1].StepName, domain.StepWaitForSourceDrain)
	}
	if steps[1].Status != domain.StepStatusRunning {
		t.Errorf("step[1].Status = %q, want running", steps[1].Status)
	}
}

func TestStepTimeline_AcquireLeaseAndPrecheck_PersistsBothSteps(t *testing.T) {
	exec := &domain.Execution{
		ID:            "step-lease-precheck-1",
		NodeName:      "gpu-01",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateNodeIdentified,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  2,
		OverallStatus: domain.OverallStatusActive,
	}
	osClient := &fakeOpenStackClient{
		computeService: &openstack.ComputeServiceStatus{Enabled: true},
	}
	orch, s := setupStepOrchestrator(t, exec, nil, osClient)
	orch.processExecution(context.Background(), exec)

	steps := requireSteps(t, s, exec.ID, 2)
	if steps[0].StepName != domain.StepAcquireLease {
		t.Errorf("step[0].StepName = %q, want %q", steps[0].StepName, domain.StepAcquireLease)
	}
	if steps[0].Status != domain.StepStatusSucceeded {
		t.Errorf("step[0].Status = %q, want succeeded", steps[0].Status)
	}
	if steps[1].StepName != domain.StepPrecheck {
		t.Errorf("step[1].StepName = %q, want %q", steps[1].StepName, domain.StepPrecheck)
	}
	if steps[1].Status != domain.StepStatusSucceeded {
		t.Errorf("step[1].Status = %q, want succeeded", steps[1].Status)
	}
}

func TestStepTimeline_CancelCleanup_SkipsWaitAndPersistsCancelStep(t *testing.T) {
	s := store.NewMemoryStore()
	exec := &domain.Execution{
		ID:                      "step-cancel-1",
		NodeName:                "gpu-01",
		Direction:               domain.DirectionSlurmToOpenStack,
		RequestedBy:             "test",
		RequestedAt:             time.Now(),
		CurrentState:            domain.StateCancelling,
		CancellationSourceState: domain.StateAwaitingSourceAllocation,
		PlaceholderJobID:        "job-99",
		DesiredOwner:            domain.OwnerOpenStack,
		PreviousOwner:           domain.OwnerSlurm,
		StateVersion:            3,
		OverallStatus:           domain.OverallStatusActive,
	}
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Simulate an open wait step
	tracker := engine.NewStepTracker(s, nil)
	_, _ = tracker.StartStep(context.Background(), exec.ID, domain.StepWaitForSourceAllocation, exec.NodeName)

	runner := engine.NewRunner(s, nil)
	orch := New(s, runner, nil, &fakeStepSlurmClient{}, nil, Config{}, nil)
	orch.processExecution(context.Background(), exec)

	steps := requireSteps(t, s, exec.ID, 2)

	if steps[0].StepName != domain.StepWaitForSourceAllocation {
		t.Errorf("step[0].StepName = %q, want %q", steps[0].StepName, domain.StepWaitForSourceAllocation)
	}
	if steps[0].Status != domain.StepStatusSkipped {
		t.Errorf("step[0].Status = %q, want skipped (closed by cancellation)", steps[0].Status)
	}
	if steps[1].StepName != domain.StepCancelCleanup {
		t.Errorf("step[1].StepName = %q, want %q", steps[1].StepName, domain.StepCancelCleanup)
	}
	if steps[1].Status != domain.StepStatusSucceeded {
		t.Errorf("step[1].Status = %q, want succeeded", steps[1].Status)
	}
}

func TestStepTimeline_FailedAction_PersistsFailedStep(t *testing.T) {
	exec := &domain.Execution{
		ID:            "step-fail-1",
		NodeName:      "gpu-01",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StatePrecheckPassed,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  3,
		OverallStatus: domain.OverallStatusActive,
	}
	slurmClient := &fakeStepSlurmClient{drainErr: errTestDrain}
	orch, s := setupStepOrchestrator(t, exec, slurmClient, nil)
	orch.processExecution(context.Background(), exec)

	steps := requireSteps(t, s, exec.ID, 1)
	if steps[0].StepName != domain.StepQuiesceSource {
		t.Errorf("step[0].StepName = %q, want %q", steps[0].StepName, domain.StepQuiesceSource)
	}
	if steps[0].Status != domain.StepStatusFailed {
		t.Errorf("step[0].Status = %q, want failed", steps[0].Status)
	}
	if steps[0].EndedAt == nil {
		t.Error("step[0].EndedAt should be set for failed steps")
	}
}

var errTestDrain = errorf("test drain failure")

type errorf string

func (e errorf) Error() string { return string(e) }
