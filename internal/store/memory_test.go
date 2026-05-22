package store

import (
	"context"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
)

func TestLeaseExclusivity(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	exec := &domain.Execution{
		ID:           "exec-1",
		NodeName:     "gpu-01",
		CurrentState: domain.StateLocked,
		StateVersion: 0,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	lease := &domain.NodeLease{
		NodeName:    "gpu-01",
		ExecutionID: "exec-1",
		Holder:      "daemon",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}
	if err := s.AcquireLease(ctx, lease); err != nil {
		t.Fatalf("first lease should succeed: %v", err)
	}

	conflictLease := &domain.NodeLease{
		NodeName:    "gpu-01",
		ExecutionID: "exec-2",
		Holder:      "daemon",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}
	if err := s.AcquireLease(ctx, conflictLease); err != ErrLeaseHeld {
		t.Fatalf("second lease should fail with ErrLeaseHeld, got: %v", err)
	}

	if err := s.ReleaseLease(ctx, "gpu-01", "exec-1"); err != nil {
		t.Fatalf("release should succeed: %v", err)
	}

	if err := s.AcquireLease(ctx, conflictLease); err != nil {
		t.Fatalf("lease after release should succeed: %v", err)
	}
}

func TestAdvanceStateVersionConflict(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	exec := &domain.Execution{
		ID:            "exec-1",
		CurrentState:  domain.StateRequested,
		StateVersion:  0,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	if err := s.AdvanceState(ctx, "exec-1", 99, domain.StateAwaitingSourceAllocation); err != ErrVersionConflict {
		t.Fatalf("expected version conflict, got: %v", err)
	}

	if err := s.AdvanceState(ctx, "exec-1", 0, domain.StateAwaitingSourceAllocation); err != nil {
		t.Fatalf("valid advance should succeed: %v", err)
	}

	got, _ := s.GetExecution(ctx, "exec-1")
	if got.StateVersion != 1 {
		t.Fatalf("state_version should be 1, got %d", got.StateVersion)
	}
	if got.CurrentState != domain.StateAwaitingSourceAllocation {
		t.Fatalf("state should be awaiting_source_allocation, got %s", got.CurrentState)
	}
}

func TestAdvanceStateRejectsInvalidTransition(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	exec := &domain.Execution{
		ID:            "exec-1",
		CurrentState:  domain.StateRequested,
		StateVersion:  0,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	if err := s.AdvanceState(ctx, "exec-1", 0, domain.StateCompleted); err != ErrVersionConflict {
		t.Fatalf("invalid transition should be rejected, got: %v", err)
	}
}
