package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

type fakeAllocationWaitSlurmClient struct {
	jobState    *slurm.JobState
	jobStateErr error
	cancelJobIDs []string
}

func (f *fakeAllocationWaitSlurmClient) SubmitPlaceholderJob(_ context.Context, _ slurm.PlaceholderJobRequest) (*slurm.PlaceholderJobResult, error) {
	return nil, errors.New("not expected in allocation wait tests")
}
func (f *fakeAllocationWaitSlurmClient) GetNodeState(_ context.Context, _ string) (*slurm.NodeState, error) {
	return nil, nil
}
func (f *fakeAllocationWaitSlurmClient) GetNodeStateWithIdentity(_ context.Context, _ string, _ slurm.WorkloadIdentity) (*slurm.NodeState, error) {
	return nil, nil
}
func (f *fakeAllocationWaitSlurmClient) DrainNode(_ context.Context, _, _ string) error { return nil }
func (f *fakeAllocationWaitSlurmClient) ResumeNode(_ context.Context, _ string) error   { return nil }
func (f *fakeAllocationWaitSlurmClient) CancelJob(_ context.Context, jobID string) error {
	f.cancelJobIDs = append(f.cancelJobIDs, jobID)
	return nil
}
func (f *fakeAllocationWaitSlurmClient) CancelJobWithIdentity(_ context.Context, jobID string, _ slurm.WorkloadIdentity) error {
	f.cancelJobIDs = append(f.cancelJobIDs, jobID)
	return nil
}
func (f *fakeAllocationWaitSlurmClient) GetJobState(_ context.Context, _ string, _ slurm.WorkloadIdentity) (*slurm.JobState, error) {
	return f.jobState, f.jobStateErr
}
func (f *fakeAllocationWaitSlurmClient) ListPartitions(_ context.Context) ([]slurm.Partition, error) {
	return nil, nil
}
func (f *fakeAllocationWaitSlurmClient) GetNodes(_ context.Context) ([]slurm.NodeState, error) {
	return nil, nil
}
func (f *fakeAllocationWaitSlurmClient) VerifyToken(_ context.Context, _, _ string) error { return nil }

func newAwaitingExec(jobID string) *domain.Execution {
	return &domain.Execution{
		ID:               "alloc-wait-exec",
		NodeName:         "",
		Direction:        domain.DirectionSlurmToOpenStack,
		RequestedBy:      "test",
		RequestedAt:      time.Now(),
		CurrentState:     domain.StateAwaitingSourceAllocation,
		PlaceholderJobID: jobID,
		DesiredOwner:     domain.OwnerOpenStack,
		PreviousOwner:    domain.OwnerSlurm,
		StateVersion:     2,
		OverallStatus:    domain.OverallStatusActive,
	}
}

func setupAllocationWaitOrchestrator(t *testing.T, exec *domain.Execution, slurmClient slurm.Client) (*Orchestrator, store.Store) {
	t.Helper()
	s := store.NewMemoryStore()
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}
	// Start the wait step as it would be in production.
	tracker := engine.NewStepTracker(s, nil)
	_, _ = tracker.StartStep(context.Background(), exec.ID, domain.StepWaitForSourceAllocation, exec.NodeName)
	runner := engine.NewRunner(s, nil)
	orch := New(s, runner, nil, slurmClient, nil, Config{}, nil)
	return orch, s
}

// TestMonitorPlaceholder_TerminalFailure tests that a placeholder job that
// reaches FAILED state causes the execution to fail as failed_non_destructive.
func TestMonitorPlaceholder_TerminalFailure(t *testing.T) {
	exec := newAwaitingExec("job-12345")
	slurmClient := &fakeAllocationWaitSlurmClient{
		jobState: &slurm.JobState{State: "FAILED", IsTerminal: true},
	}
	orch, s := setupAllocationWaitOrchestrator(t, exec, slurmClient)

	orch.processExecution(context.Background(), exec)

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateFailedNonDestructive {
		t.Fatalf("state = %s, want failed_non_destructive", updated.CurrentState)
	}
	if updated.FinalErrorCode != "placeholder_job_failed" {
		t.Fatalf("final_error_code = %q, want placeholder_job_failed", updated.FinalErrorCode)
	}
	if updated.FinalErrorSummary == "" {
		t.Fatal("final_error_summary should be set")
	}

	steps, _ := s.ListSteps(context.Background(), exec.ID)
	var waitStep *domain.StepRecord
	for _, step := range steps {
		if step.StepName == domain.StepWaitForSourceAllocation {
			waitStep = step
		}
	}
	if waitStep == nil {
		t.Fatal("expected wait_for_source_allocation step")
	}
	if waitStep.Status != domain.StepStatusFailed {
		t.Fatalf("wait step status = %s, want failed", waitStep.Status)
	}
	if waitStep.ErrorSummary == "" {
		t.Fatal("wait step error_summary should be set")
	}
}

// TestMonitorPlaceholder_CompletedWithoutAllocation tests that COMPLETED
// state before any allocation event is also treated as failure.
func TestMonitorPlaceholder_CompletedWithoutAllocation(t *testing.T) {
	exec := newAwaitingExec("job-12345")
	slurmClient := &fakeAllocationWaitSlurmClient{
		jobState: &slurm.JobState{State: "COMPLETED", IsTerminal: true},
	}
	orch, s := setupAllocationWaitOrchestrator(t, exec, slurmClient)

	orch.processExecution(context.Background(), exec)

	updated, _ := s.GetExecution(context.Background(), exec.ID)
	if updated.CurrentState != domain.StateFailedNonDestructive {
		t.Fatalf("state = %s, want failed_non_destructive (COMPLETED without allocation = failure)", updated.CurrentState)
	}
	if updated.FinalErrorSummary == "" {
		t.Fatal("final_error_summary should be set")
	}
}

// TestMonitorPlaceholder_NonTerminal tests that a non-terminal job state
// does not advance the execution.
func TestMonitorPlaceholder_NonTerminal(t *testing.T) {
	exec := newAwaitingExec("job-12345")
	slurmClient := &fakeAllocationWaitSlurmClient{
		jobState: &slurm.JobState{State: "RUNNING", IsTerminal: false},
	}
	orch, s := setupAllocationWaitOrchestrator(t, exec, slurmClient)

	orch.processExecution(context.Background(), exec)

	updated, _ := s.GetExecution(context.Background(), exec.ID)
	if updated.CurrentState != domain.StateAwaitingSourceAllocation {
		t.Fatalf("state = %s, want awaiting_source_allocation (still running)", updated.CurrentState)
	}
	if updated.OverallStatus != domain.OverallStatusActive {
		t.Fatalf("overall_status = %s, want active", updated.OverallStatus)
	}
}

// TestMonitorPlaceholder_PendingState tests that a PENDING job is not treated as failed.
func TestMonitorPlaceholder_PendingState(t *testing.T) {
	exec := newAwaitingExec("job-12345")
	slurmClient := &fakeAllocationWaitSlurmClient{
		jobState: &slurm.JobState{State: "PENDING", IsTerminal: false},
	}
	orch, s := setupAllocationWaitOrchestrator(t, exec, slurmClient)

	orch.processExecution(context.Background(), exec)

	updated, _ := s.GetExecution(context.Background(), exec.ID)
	if updated.CurrentState != domain.StateAwaitingSourceAllocation {
		t.Fatalf("state = %s, want awaiting_source_allocation (still pending)", updated.CurrentState)
	}
}

// TestMonitorPlaceholder_LateAllocationRace tests that when an allocation MQ
// event arrives before the monitor runs, the state-version guard prevents
// double-processing. The execution has already advanced to node_identified so
// the monitor action is never taken (determineS2O returns actionNone).
func TestMonitorPlaceholder_LateAllocationRace(t *testing.T) {
	s := store.NewMemoryStore()
	exec := newAwaitingExec("job-12345")
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	// Simulate a concurrent MQ allocation event advancing the execution.
	if err := s.AdvanceState(context.Background(), exec.ID, exec.StateVersion, domain.StateNodeIdentified); err != nil {
		t.Fatalf("advance state: %v", err)
	}
	advanced, _ := s.GetExecution(context.Background(), exec.ID)
	advanced.NodeName = "gpu-node-01"
	_ = s.UpdateExecution(context.Background(), advanced)

	slurmClient := &fakeAllocationWaitSlurmClient{
		jobState: &slurm.JobState{State: "COMPLETED", IsTerminal: true},
	}
	runner := engine.NewRunner(s, nil)
	orch := New(s, runner, nil, slurmClient, nil, Config{}, nil)

	// Use the stale exec (still at awaiting_source_allocation). Since the
	// fresh read inside runExecution will see node_identified, the worker
	// exits without calling processExecution again with the terminal state.
	orch.processExecution(context.Background(), exec)

	// The execution should remain at node_identified — not reverted.
	updated, _ := s.GetExecution(context.Background(), exec.ID)
	if updated.CurrentState != domain.StateNodeIdentified {
		t.Fatalf("state = %s, want node_identified (late allocation should win)", updated.CurrentState)
	}
}

// TestRecovery_AwaitingAllocationWithJobID tests that an execution in
// awaiting_source_allocation with a placeholder job is recovered on startup.
func TestRecovery_AwaitingAllocationWithJobID(t *testing.T) {
	s := store.NewMemoryStore()
	exec := newAwaitingExec("job-99")
	exec.ID = "recover-alloc-exec"
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	slurmClient := &fakeAllocationWaitSlurmClient{
		jobState: &slurm.JobState{State: "FAILED", IsTerminal: true},
	}
	runner := engine.NewRunner(s, nil)
	orch := New(s, runner, nil, slurmClient, nil, Config{TickInterval: 20 * time.Millisecond}, nil)

	runCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	orch.Run(runCtx)

	updated, _ := s.GetExecution(context.Background(), exec.ID)
	if updated.CurrentState != domain.StateFailedNonDestructive {
		t.Fatalf("state = %s, want failed_non_destructive after recovery detected terminal placeholder", updated.CurrentState)
	}
}

// TestRecovery_AwaitingAllocationWithoutJobID tests that an execution in
// awaiting_source_allocation without a placeholder job is NOT recovered
// (it has no job to poll so it would be a pure MQ wait).
func TestRecovery_AwaitingAllocationWithoutJobID(t *testing.T) {
	s := store.NewMemoryStore()
	exec := newAwaitingExec("") // no placeholder job
	exec.ID = "recover-alloc-nojob"
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	slurmClient := &fakeAllocationWaitSlurmClient{}
	runner := engine.NewRunner(s, nil)
	orch := New(s, runner, nil, slurmClient, nil, Config{TickInterval: 20 * time.Millisecond}, nil)

	runCtx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	orch.Run(runCtx)

	updated, _ := s.GetExecution(context.Background(), exec.ID)
	if updated.CurrentState != domain.StateAwaitingSourceAllocation {
		t.Fatalf("state = %s, want awaiting_source_allocation (no job to poll)", updated.CurrentState)
	}
}
