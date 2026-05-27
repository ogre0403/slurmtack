package service

import (
	"context"
	"testing"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
)

func TestRequestSwitchPersistsSlurmPartition(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil)

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:       domain.DirectionSlurmToOpenStack,
		RequestedBy:     "operator-1",
		SlurmConstraint: "gpu-a100",
		SlurmPartition:  "gpu-maint",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}

	exec, err := s.GetExecution(context.Background(), id)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if exec.RequestedSlurmConstraint != "gpu-a100" {
		t.Fatalf("RequestedSlurmConstraint = %q, want gpu-a100", exec.RequestedSlurmConstraint)
	}
	if exec.RequestedSlurmPartition != "gpu-maint" {
		t.Fatalf("RequestedSlurmPartition = %q, want gpu-maint", exec.RequestedSlurmPartition)
	}
}

func TestRequestSwitchLeavesSlurmPartitionEmptyWhenOmitted(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil)

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:       domain.DirectionSlurmToOpenStack,
		RequestedBy:     "operator-1",
		SlurmConstraint: "gpu-a100",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}

	exec, err := s.GetExecution(context.Background(), id)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if exec.RequestedSlurmPartition != "" {
		t.Fatalf("RequestedSlurmPartition = %q, want empty string", exec.RequestedSlurmPartition)
	}
}
