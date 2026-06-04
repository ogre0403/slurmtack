package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
)

func createExecInState(t *testing.T, s store.Store, state domain.SwitchState, dir domain.SwitchDirection) *domain.Execution {
	t.Helper()
	exec := &domain.Execution{
		ID:            "test-exec",
		NodeName:      "gpu-01",
		Direction:     dir,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  state,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  0,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}
	return exec
}

func TestCancelSwitch_AcceptsAwaitingTargetNode(t *testing.T) {
	s := store.NewMemoryStore()
	createExecInState(t, s, domain.StateAwaitingTargetNode, domain.DirectionOpenStackToSlurm)
	svc := NewSwitchService(s, nil)

	if err := svc.CancelSwitch(context.Background(), "test-exec"); err != nil {
		t.Fatalf("CancelSwitch() error = %v", err)
	}

	exec, _ := s.GetExecution(context.Background(), "test-exec")
	if exec.CurrentState != domain.StateCancelling {
		t.Fatalf("state = %s, want cancelling", exec.CurrentState)
	}
	if exec.CancellationSourceState != domain.StateAwaitingTargetNode {
		t.Fatalf("cancellation_source_state = %s, want awaiting_target_node", exec.CancellationSourceState)
	}
}

func TestCancelSwitch_AcceptsAwaitingSourceAllocation(t *testing.T) {
	s := store.NewMemoryStore()
	createExecInState(t, s, domain.StateAwaitingSourceAllocation, domain.DirectionSlurmToOpenStack)
	svc := NewSwitchService(s, nil)

	if err := svc.CancelSwitch(context.Background(), "test-exec"); err != nil {
		t.Fatalf("CancelSwitch() error = %v", err)
	}

	exec, _ := s.GetExecution(context.Background(), "test-exec")
	if exec.CurrentState != domain.StateCancelling {
		t.Fatalf("state = %s, want cancelling", exec.CurrentState)
	}
	if exec.CancellationSourceState != domain.StateAwaitingSourceAllocation {
		t.Fatalf("cancellation_source_state = %s, want awaiting_source_allocation", exec.CancellationSourceState)
	}
}

func TestCancelSwitch_AcceptsSourceQuiescing(t *testing.T) {
	s := store.NewMemoryStore()
	createExecInState(t, s, domain.StateSourceQuiescing, domain.DirectionSlurmToOpenStack)
	svc := NewSwitchService(s, nil)

	if err := svc.CancelSwitch(context.Background(), "test-exec"); err != nil {
		t.Fatalf("CancelSwitch() error = %v", err)
	}

	exec, _ := s.GetExecution(context.Background(), "test-exec")
	if exec.CurrentState != domain.StateCancelling {
		t.Fatalf("state = %s, want cancelling", exec.CurrentState)
	}
}

func TestCancelSwitch_RejectsNonCancellableState(t *testing.T) {
	nonCancellable := []domain.SwitchState{
		domain.StateRequested,
		domain.StateLocked,
		domain.StatePrecheckPassed,
		domain.StateRebooting,
		domain.StateVerifying,
		domain.StateCompleted,
	}

	for _, state := range nonCancellable {
		t.Run(string(state), func(t *testing.T) {
			s := store.NewMemoryStore()
			exec := &domain.Execution{
				ID:            "test-exec",
				NodeName:      "gpu-01",
				Direction:     domain.DirectionSlurmToOpenStack,
				RequestedBy:   "test",
				RequestedAt:   time.Now(),
				CurrentState:  state,
				DesiredOwner:  domain.OwnerOpenStack,
				PreviousOwner: domain.OwnerSlurm,
				StateVersion:  0,
				OverallStatus: domain.OverallStatusActive,
			}
			_ = s.CreateExecution(context.Background(), exec)
			svc := NewSwitchService(s, nil)

			err := svc.CancelSwitch(context.Background(), "test-exec")
			if !errors.Is(err, ErrCancelNotAllowed) {
				t.Fatalf("CancelSwitch() error = %v, want ErrCancelNotAllowed", err)
			}

			fresh, _ := s.GetExecution(context.Background(), "test-exec")
			if fresh.CurrentState != state {
				t.Fatalf("state changed to %s, want %s unchanged", fresh.CurrentState, state)
			}
		})
	}
}

func TestCancelSwitch_IdempotentWhenAlreadyCancelling(t *testing.T) {
	s := store.NewMemoryStore()
	exec := &domain.Execution{
		ID:                      "test-exec",
		NodeName:                "gpu-01",
		Direction:               domain.DirectionSlurmToOpenStack,
		RequestedBy:             "test",
		RequestedAt:             time.Now(),
		CurrentState:            domain.StateCancelling,
		CancellationSourceState: domain.StateAwaitingSourceAllocation,
		DesiredOwner:            domain.OwnerOpenStack,
		PreviousOwner:           domain.OwnerSlurm,
		StateVersion:            1,
		OverallStatus:           domain.OverallStatusActive,
	}
	_ = s.CreateExecution(context.Background(), exec)
	svc := NewSwitchService(s, nil)

	// Second call must be a no-op.
	if err := svc.CancelSwitch(context.Background(), "test-exec"); err != nil {
		t.Fatalf("CancelSwitch() idempotent call error = %v", err)
	}

	fresh, _ := s.GetExecution(context.Background(), "test-exec")
	if fresh.CurrentState != domain.StateCancelling {
		t.Fatalf("state = %s after idempotent cancel, want cancelling", fresh.CurrentState)
	}
}

func TestCancelSwitch_IdempotentWhenAlreadyCancelled(t *testing.T) {
	s := store.NewMemoryStore()
	exec := &domain.Execution{
		ID:            "test-exec",
		NodeName:      "gpu-01",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateCancelled,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  2,
		OverallStatus: domain.OverallStatusFailed,
	}
	_ = s.CreateExecution(context.Background(), exec)
	svc := NewSwitchService(s, nil)

	if err := svc.CancelSwitch(context.Background(), "test-exec"); err != nil {
		t.Fatalf("CancelSwitch() idempotent call on cancelled error = %v", err)
	}
}

func TestCancelSwitch_NotFound(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil)

	err := svc.CancelSwitch(context.Background(), "nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("CancelSwitch() error = %v, want ErrNotFound", err)
	}
}
