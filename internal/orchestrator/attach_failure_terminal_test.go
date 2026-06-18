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

// newHostReachableExecution builds an execution parked in host_reachable, the
// state the orchestrator drives attach from in both switch directions.
func newHostReachableExecution(direction domain.SwitchDirection) *domain.Execution {
	desired, previous := domain.OwnerSlurm, domain.OwnerOpenStack
	if direction == domain.DirectionSlurmToOpenStack {
		desired, previous = domain.OwnerOpenStack, domain.OwnerSlurm
	}
	return &domain.Execution{
		ID:            "attach-fail-" + string(direction),
		NodeName:      "gpu-node-01",
		Direction:     direction,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateHostReachable,
		DesiredOwner:  desired,
		PreviousOwner: previous,
		StateVersion:  1,
		OverallStatus: domain.OverallStatusActive,
	}
}

// TestProcessExecution_OpenStackToSlurm_AttachFailureTerminalizes proves an
// attach failure from host_reachable (before target_attaching is persisted)
// ends in failed_needs_rollback instead of leaving the execution active.
func TestProcessExecution_OpenStackToSlurm_AttachFailureTerminalizes(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()
	exec := newHostReachableExecution(domain.DirectionOpenStackToSlurm)
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution() error = %v", err)
	}

	runner := engine.NewRunner(s, nil)
	sshRunner := &recordingSSHRunner{}
	// An unsupported node state makes EnsureNodeReadyForAttach fail after the
	// slurmd restore commands succeed.
	client := &attachTestSlurmClient{
		nodeState: &slurm.NodeState{NodeName: exec.NodeName, State: "fail"},
	}
	orch := New(s, runner, sshRunner, client, nil, Config{}, nil)

	orch.processExecution(ctx, exec)

	updated, err := s.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if updated.CurrentState != domain.StateFailedNeedsRollback {
		t.Fatalf("CurrentState = %s, want %s", updated.CurrentState, domain.StateFailedNeedsRollback)
	}
	if !updated.CurrentState.IsTerminal() {
		t.Fatalf("state %s should be terminal", updated.CurrentState)
	}
	if updated.OverallStatus == domain.OverallStatusActive {
		t.Fatalf("OverallStatus = %s, execution should not remain active", updated.OverallStatus)
	}
}

// TestProcessExecution_SlurmToOpenStack_AttachFailureTerminalizes proves the
// same contract for the OpenStack compute-service enable attach path.
func TestProcessExecution_SlurmToOpenStack_AttachFailureTerminalizes(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()
	exec := newHostReachableExecution(domain.DirectionSlurmToOpenStack)
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution() error = %v", err)
	}

	runner := engine.NewRunner(s, nil)
	osClient := &fakeOpenStackClient{enableComputeErr: errors.New("compute service enable rejected")}
	orch := New(s, runner, nil, nil, osClient, Config{}, nil)

	orch.processExecution(ctx, exec)

	if osClient.enableComputeCalls != 1 {
		t.Fatalf("EnableComputeService calls = %d, want 1", osClient.enableComputeCalls)
	}

	updated, err := s.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if updated.CurrentState != domain.StateFailedNeedsRollback {
		t.Fatalf("CurrentState = %s, want %s", updated.CurrentState, domain.StateFailedNeedsRollback)
	}
	if !updated.CurrentState.IsTerminal() {
		t.Fatalf("state %s should be terminal", updated.CurrentState)
	}
	if updated.OverallStatus == domain.OverallStatusActive {
		t.Fatalf("OverallStatus = %s, execution should not remain active", updated.OverallStatus)
	}
}
