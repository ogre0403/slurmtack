package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
)

func TestEventHandlerRejectsDuplicateVersion(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()

	exec := &domain.Execution{
		ID:            "exec-1",
		CurrentState:  domain.StateSourceQuiescing,
		StateVersion:  3,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(s)
	eh := NewEventHandler(s, r)

	err := eh.Handle(ctx, Event{
		Type:         "drained",
		ExecutionID:  "exec-1",
		StateVersion: 1,
	})
	if err == nil {
		t.Fatal("expected error for stale event")
	}
	if !strings.Contains(err.Error(), "stale event") {
		t.Fatalf("expected stale event error, got: %v", err)
	}
}

func TestEventHandlerRejectsWrongState(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()

	exec := &domain.Execution{
		ID:            "exec-1",
		CurrentState:  domain.StateLocked,
		StateVersion:  2,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(s)
	eh := NewEventHandler(s, r)

	err := eh.Handle(ctx, Event{
		Type:         "drained",
		ExecutionID:  "exec-1",
		StateVersion: 2,
	})
	if err == nil {
		t.Fatal("expected error for wrong-state event")
	}
	if !strings.Contains(err.Error(), "not expected") {
		t.Fatalf("expected 'not expected' error, got: %v", err)
	}
}

func TestEventHandlerAcceptsValidDrained(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()

	exec := &domain.Execution{
		ID:            "exec-1",
		CurrentState:  domain.StateSourceQuiescing,
		StateVersion:  3,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(s)
	eh := NewEventHandler(s, r)

	err := eh.Handle(ctx, Event{
		Type:         "drained",
		ExecutionID:  "exec-1",
		StateVersion: 3,
	})
	if err != nil {
		t.Fatalf("valid drained event should succeed: %v", err)
	}

	got, _ := s.GetExecution(ctx, "exec-1")
	if got.CurrentState != domain.StateSourceDetached {
		t.Fatalf("state should be source_detached, got %s", got.CurrentState)
	}
}

func TestEventHandlerRejectsTerminalExecution(t *testing.T) {
	s := store.NewMemoryStore()
	ctx := context.Background()

	exec := &domain.Execution{
		ID:            "exec-1",
		CurrentState:  domain.StateCompleted,
		StateVersion:  5,
		OverallStatus: domain.OverallStatusSucceeded,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(s)
	eh := NewEventHandler(s, r)

	err := eh.Handle(ctx, Event{
		Type:         "host_reachable",
		ExecutionID:  "exec-1",
		StateVersion: 5,
	})
	if err == nil {
		t.Fatal("expected error for terminal execution")
	}
	if !strings.Contains(err.Error(), "terminal") {
		t.Fatalf("expected terminal error, got: %v", err)
	}
}
