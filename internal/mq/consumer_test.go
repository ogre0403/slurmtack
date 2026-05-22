package mq_test

import (
	"context"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
)

func TestListActiveExecutions(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()

	exec := &domain.Execution{
		ID:            "exec-1",
		NodeName:      "gpu-node-01",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateAwaitingSourceAllocation,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  1,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	completedExec := &domain.Execution{
		ID:            "exec-2",
		NodeName:      "gpu-node-02",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateCompleted,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  5,
		OverallStatus: domain.OverallStatusSucceeded,
	}
	if err := s.CreateExecution(ctx, completedExec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	active, err := s.ListActiveExecutions(ctx)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}

	if len(active) != 1 {
		t.Fatalf("expected 1 active execution, got %d", len(active))
	}
	if active[0].ID != "exec-1" {
		t.Errorf("expected exec-1, got %s", active[0].ID)
	}
}
