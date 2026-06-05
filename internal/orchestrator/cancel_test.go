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

// fakeSlurmCancelClient records calls for cancel-specific tests.
type fakeSlurmCancelClient struct {
	resumeErr     error
	cancelJobErr  error
	resumeCalled  bool
	cancelJobIDs  []string
}

func (f *fakeSlurmCancelClient) SubmitPlaceholderJob(_ context.Context, _ slurm.PlaceholderJobRequest) (*slurm.PlaceholderJobResult, error) {
	return nil, errors.New("not expected in cancel tests")
}

func (f *fakeSlurmCancelClient) GetNodeState(_ context.Context, _ string) (*slurm.NodeState, error) {
	return nil, nil
}

func (f *fakeSlurmCancelClient) DrainNode(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeSlurmCancelClient) ResumeNode(_ context.Context, _ string) error {
	f.resumeCalled = true
	return f.resumeErr
}

func (f *fakeSlurmCancelClient) CancelJob(_ context.Context, jobID string) error {
	f.cancelJobIDs = append(f.cancelJobIDs, jobID)
	return f.cancelJobErr
}

func (f *fakeSlurmCancelClient) ListPartitions(_ context.Context) ([]slurm.Partition, error) {
	return nil, nil
}

func newCancellingExec(direction domain.SwitchDirection, sourceState domain.SwitchState, jobID string) *domain.Execution {
	return &domain.Execution{
		ID:                      "cancel-exec-1",
		NodeName:                "gpu-node-01",
		Direction:               direction,
		RequestedBy:             "test",
		RequestedAt:             time.Now(),
		CurrentState:            domain.StateCancelling,
		CancellationSourceState: sourceState,
		PlaceholderJobID:        jobID,
		DesiredOwner:            domain.OwnerOpenStack,
		PreviousOwner:           domain.OwnerSlurm,
		StateVersion:            2,
		OverallStatus:           domain.OverallStatusActive,
	}
}

func setupCancelOrchestrator(t *testing.T, exec *domain.Execution, slurmClient slurm.Client, osClient *fakeOpenStackClient) (*Orchestrator, store.Store) {
	t.Helper()
	s := store.NewMemoryStore()
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}
	runner := engine.NewRunner(s, nil)
	orch := New(s, runner, nil, slurmClient, osClient, Config{}, nil)
	return orch, s
}

// --- awaiting_target_node: no external cleanup ---

func TestCancelCleanup_AwaitingTargetNode_TransitionsToCancelled(t *testing.T) {
	exec := newCancellingExec(domain.DirectionOpenStackToSlurm, domain.StateAwaitingTargetNode, "")
	orch, s := setupCancelOrchestrator(t, exec, nil, nil)

	orch.processExecution(context.Background(), exec)

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateCancelled {
		t.Fatalf("state = %s, want %s", updated.CurrentState, domain.StateCancelled)
	}
	if updated.OverallStatus != domain.OverallStatusFailed {
		t.Fatalf("overall_status = %s, want failed", updated.OverallStatus)
	}
	if updated.FinalErrorCode != "cancelled_by_user" {
		t.Fatalf("final_error_code = %q, want cancelled_by_user", updated.FinalErrorCode)
	}
}

// --- awaiting_source_allocation: cancel placeholder job ---

func TestCancelCleanup_AwaitingSourceAllocation_CancelsPlaceholderJob(t *testing.T) {
	exec := newCancellingExec(domain.DirectionSlurmToOpenStack, domain.StateAwaitingSourceAllocation, "job-99")
	slurmClient := &fakeSlurmCancelClient{}
	orch, s := setupCancelOrchestrator(t, exec, slurmClient, nil)

	orch.processExecution(context.Background(), exec)

	updated, _ := s.GetExecution(context.Background(), exec.ID)
	if updated.CurrentState != domain.StateCancelled {
		t.Fatalf("state = %s, want cancelled", updated.CurrentState)
	}
	if len(slurmClient.cancelJobIDs) != 1 || slurmClient.cancelJobIDs[0] != "job-99" {
		t.Fatalf("CancelJob calls = %v, want [job-99]", slurmClient.cancelJobIDs)
	}
}

func TestCancelCleanup_AwaitingSourceAllocation_NoJobID_SkipsCancelJob(t *testing.T) {
	exec := newCancellingExec(domain.DirectionSlurmToOpenStack, domain.StateAwaitingSourceAllocation, "")
	slurmClient := &fakeSlurmCancelClient{}
	orch, s := setupCancelOrchestrator(t, exec, slurmClient, nil)

	orch.processExecution(context.Background(), exec)

	updated, _ := s.GetExecution(context.Background(), exec.ID)
	if updated.CurrentState != domain.StateCancelled {
		t.Fatalf("state = %s, want cancelled", updated.CurrentState)
	}
	if len(slurmClient.cancelJobIDs) != 0 {
		t.Fatalf("expected no CancelJob calls, got %v", slurmClient.cancelJobIDs)
	}
}

// --- source_quiescing slurm_to_openstack: resume node, cancel job, release lease ---

func TestCancelCleanup_SourceQuiescing_S2O_ResumesNodeAndCancelsJob(t *testing.T) {
	exec := newCancellingExec(domain.DirectionSlurmToOpenStack, domain.StateSourceQuiescing, "job-42")
	// Put a lease in the store so ReleaseLease has something to release.
	slurmClient := &fakeSlurmCancelClient{}
	orch, s := setupCancelOrchestrator(t, exec, slurmClient, nil)
	_ = s.AcquireLease(context.Background(), &domain.NodeLease{
		NodeName:    "gpu-node-01",
		ExecutionID: exec.ID,
		Holder:      "orchestrator",
		ExpiresAt:   time.Now().Add(time.Hour),
	})

	orch.processExecution(context.Background(), exec)

	updated, _ := s.GetExecution(context.Background(), exec.ID)
	if updated.CurrentState != domain.StateCancelled {
		t.Fatalf("state = %s, want cancelled", updated.CurrentState)
	}
	if !slurmClient.resumeCalled {
		t.Fatal("expected ResumeNode to be called")
	}
	if len(slurmClient.cancelJobIDs) != 1 || slurmClient.cancelJobIDs[0] != "job-42" {
		t.Fatalf("CancelJob calls = %v, want [job-42]", slurmClient.cancelJobIDs)
	}
	_, leaseErr := s.GetLease(context.Background(), "gpu-node-01")
	if leaseErr == nil {
		t.Fatal("expected lease to be released")
	}
}

// --- source_quiescing openstack_to_slurm: re-enable compute, release lease ---

func TestCancelCleanup_SourceQuiescing_O2S_ReenablesComputeAndReleasesLease(t *testing.T) {
	exec := newCancellingExec(domain.DirectionOpenStackToSlurm, domain.StateSourceQuiescing, "")
	osClient := &fakeOpenStackClient{}
	orch, s := setupCancelOrchestrator(t, exec, nil, osClient)
	_ = s.AcquireLease(context.Background(), &domain.NodeLease{
		NodeName:    "gpu-node-01",
		ExecutionID: exec.ID,
		Holder:      "orchestrator",
		ExpiresAt:   time.Now().Add(time.Hour),
	})

	orch.processExecution(context.Background(), exec)

	updated, _ := s.GetExecution(context.Background(), exec.ID)
	if updated.CurrentState != domain.StateCancelled {
		t.Fatalf("state = %s, want cancelled", updated.CurrentState)
	}
	if osClient.enableComputeCalls != 1 {
		t.Fatalf("EnableComputeService calls = %d, want 1", osClient.enableComputeCalls)
	}
	_, leaseErr := s.GetLease(context.Background(), "gpu-node-01")
	if leaseErr == nil {
		t.Fatal("expected lease to be released")
	}
}

// --- cleanup failure → failed_non_destructive with cancel_cleanup_failed ---

func TestCancelCleanup_Failure_TerminalizesAsFailedNonDestructive(t *testing.T) {
	exec := newCancellingExec(domain.DirectionSlurmToOpenStack, domain.StateSourceQuiescing, "")
	slurmClient := &fakeSlurmCancelClient{resumeErr: errors.New("slurm down")}
	orch, s := setupCancelOrchestrator(t, exec, slurmClient, nil)

	orch.processExecution(context.Background(), exec)

	updated, _ := s.GetExecution(context.Background(), exec.ID)
	if updated.CurrentState != domain.StateFailedNonDestructive {
		t.Fatalf("state = %s, want failed_non_destructive", updated.CurrentState)
	}
	if updated.FinalErrorCode != "cancel_cleanup_failed" {
		t.Fatalf("final_error_code = %q, want cancel_cleanup_failed", updated.FinalErrorCode)
	}
}

// --- recovery: cancelling execution is re-armed on startup ---

func TestRecovery_CancellingExecutionIsRecovered(t *testing.T) {
	exec := newCancellingExec(domain.DirectionOpenStackToSlurm, domain.StateAwaitingTargetNode, "")
	s := store.NewMemoryStore()
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	runner := engine.NewRunner(s, nil)
	orch := New(s, runner, nil, nil, nil, Config{TickInterval: 50 * time.Millisecond}, nil)

	runCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	orch.Run(runCtx)

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateCancelled {
		t.Fatalf("state = %s, want cancelled after recovery", updated.CurrentState)
	}
}

// --- late MQ event race: state check discards stale events on cancelling execution ---

func TestLateEvent_CancellingExecutionIgnoresNormalProgressionAttempt(t *testing.T) {
	// Simulate what happens if an MQ consumer tries to advance a cancelling execution.
	// AdvanceState uses optimistic locking; a transition from cancelling to node_identified
	// is not in the allowed map, so it should be rejected.
	s := store.NewMemoryStore()
	exec := newCancellingExec(domain.DirectionSlurmToOpenStack, domain.StateAwaitingSourceAllocation, "")
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	// Attempt a stale "allocation complete" advance to node_identified — not valid from cancelling.
	err := s.AdvanceState(context.Background(), exec.ID, exec.StateVersion, domain.StateNodeIdentified)
	if err == nil {
		t.Fatal("expected AdvanceState to fail from cancelling to node_identified")
	}

	// State must remain cancelling.
	fresh, _ := s.GetExecution(context.Background(), exec.ID)
	if fresh.CurrentState != domain.StateCancelling {
		t.Fatalf("state = %s after stale event, want cancelling", fresh.CurrentState)
	}
}
