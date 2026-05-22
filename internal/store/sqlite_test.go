package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	f, err := os.CreateTemp("", "slurmtack-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	s, err := NewSQLiteStore(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndGetExecution(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	exec := &domain.Execution{
		ID:            "exec-1",
		NodeName:      "gpu-01",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "operator",
		RequestedAt:   now,
		CurrentState:  domain.StateRequested,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  0,
		OverallStatus: domain.OverallStatusActive,
	}

	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetExecution(ctx, "exec-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "exec-1" || got.NodeName != "gpu-01" || got.Direction != domain.DirectionSlurmToOpenStack {
		t.Fatalf("unexpected execution: %+v", got)
	}
	if got.OverallStatus != domain.OverallStatusActive {
		t.Fatalf("expected active, got %s", got.OverallStatus)
	}
}

func TestGetExecutionNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetExecution(context.Background(), "nonexistent")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListExecutions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	for _, name := range []string{"gpu-01", "gpu-02", "gpu-01"} {
		exec := &domain.Execution{
			ID: "exec-" + name + "-" + time.Now().Format("150405.000"),
			NodeName: name, Direction: domain.DirectionSlurmToOpenStack,
			RequestedBy: "op", RequestedAt: now, CurrentState: domain.StateRequested,
			DesiredOwner: domain.OwnerOpenStack, PreviousOwner: domain.OwnerSlurm,
			OverallStatus: domain.OverallStatusActive,
		}
		s.CreateExecution(ctx, exec)
		time.Sleep(time.Millisecond)
	}

	all, err := s.ListExecutions(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}

	filtered, err := s.ListExecutions(ctx, "gpu-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2, got %d", len(filtered))
	}
}

func TestAdvanceState(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	exec := &domain.Execution{
		ID: "exec-adv", NodeName: "gpu-01", Direction: domain.DirectionSlurmToOpenStack,
		RequestedBy: "op", RequestedAt: now, CurrentState: domain.StateRequested,
		DesiredOwner: domain.OwnerOpenStack, PreviousOwner: domain.OwnerSlurm,
		OverallStatus: domain.OverallStatusActive,
	}
	s.CreateExecution(ctx, exec)

	if err := s.AdvanceState(ctx, "exec-adv", 0, domain.StateNodeIdentified); err != nil {
		t.Fatal(err)
	}

	got, _ := s.GetExecution(ctx, "exec-adv")
	if got.CurrentState != domain.StateNodeIdentified || got.StateVersion != 1 {
		t.Fatalf("unexpected state: %s v%d", got.CurrentState, got.StateVersion)
	}

	// Stale version conflict
	if err := s.AdvanceState(ctx, "exec-adv", 0, domain.StateLocked); err != ErrVersionConflict {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
}

func TestAdvanceStateTerminal(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	exec := &domain.Execution{
		ID: "exec-term", NodeName: "gpu-01", Direction: domain.DirectionSlurmToOpenStack,
		RequestedBy: "op", RequestedAt: now, CurrentState: domain.StateRequested,
		DesiredOwner: domain.OwnerOpenStack, PreviousOwner: domain.OwnerSlurm,
		OverallStatus: domain.OverallStatusActive,
	}
	s.CreateExecution(ctx, exec)

	s.AdvanceState(ctx, "exec-term", 0, domain.StateFailedNonDestructive)
	got, _ := s.GetExecution(ctx, "exec-term")
	if got.OverallStatus != domain.OverallStatusFailed {
		t.Fatalf("expected failed, got %s", got.OverallStatus)
	}
}

func TestUpdateExecution(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	exec := &domain.Execution{
		ID: "exec-upd", NodeName: "", Direction: domain.DirectionSlurmToOpenStack,
		RequestedBy: "op", RequestedAt: now, CurrentState: domain.StateRequested,
		DesiredOwner: domain.OwnerOpenStack, PreviousOwner: domain.OwnerSlurm,
		OverallStatus: domain.OverallStatusActive,
	}
	s.CreateExecution(ctx, exec)

	exec.NodeName = "gpu-05"
	exec.PlaceholderJobID = "job-123"
	if err := s.UpdateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	got, _ := s.GetExecution(ctx, "exec-upd")
	if got.NodeName != "gpu-05" || got.PlaceholderJobID != "job-123" {
		t.Fatalf("update not persisted: %+v", got)
	}
}

func TestLeaseLifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	lease := &domain.NodeLease{
		NodeName:    "gpu-01",
		ExecutionID: "exec-a",
		Holder:      "daemon-1",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	if err := s.AcquireLease(ctx, lease); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetLease(ctx, "gpu-01")
	if err != nil {
		t.Fatal(err)
	}
	if got.ExecutionID != "exec-a" {
		t.Fatalf("unexpected lease: %+v", got)
	}

	// Different execution blocked
	lease2 := &domain.NodeLease{
		NodeName:    "gpu-01",
		ExecutionID: "exec-b",
		Holder:      "daemon-1",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}
	if err := s.AcquireLease(ctx, lease2); err != ErrLeaseHeld {
		t.Fatalf("expected ErrLeaseHeld, got %v", err)
	}

	// Release
	if err := s.ReleaseLease(ctx, "gpu-01", "exec-a"); err != nil {
		t.Fatal(err)
	}

	// Now acquire succeeds
	if err := s.AcquireLease(ctx, lease2); err != nil {
		t.Fatal(err)
	}
}

func TestLeaseNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetLease(context.Background(), "nonexistent")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestReleaseLeaseNotHeld(t *testing.T) {
	s := newTestStore(t)
	err := s.ReleaseLease(context.Background(), "gpu-01", "exec-x")
	if err != ErrLeaseNotHeld {
		t.Fatalf("expected ErrLeaseNotHeld, got %v", err)
	}
}

func TestStepLifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	// Create execution first for FK
	exec := &domain.Execution{
		ID: "exec-steps", NodeName: "gpu-01", Direction: domain.DirectionSlurmToOpenStack,
		RequestedBy: "op", RequestedAt: now, CurrentState: domain.StateRequested,
		DesiredOwner: domain.OwnerOpenStack, PreviousOwner: domain.OwnerSlurm,
		OverallStatus: domain.OverallStatusActive,
	}
	s.CreateExecution(ctx, exec)

	step1 := &domain.StepRecord{
		ExecutionID: "exec-steps", StepName: "drain_node", Sequence: 1,
		Host: "gpu-01", StartedAt: now, Status: domain.StepStatusRunning,
	}
	step2 := &domain.StepRecord{
		ExecutionID: "exec-steps", StepName: "detach_network", Sequence: 2,
		Host: "gpu-01", StartedAt: now, Status: domain.StepStatusPending,
	}

	if err := s.CreateStep(ctx, step1); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateStep(ctx, step2); err != nil {
		t.Fatal(err)
	}

	steps, err := s.ListSteps(ctx, "exec-steps")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].Sequence != 1 || steps[1].Sequence != 2 {
		t.Fatal("steps not ordered by sequence")
	}

	// Update step
	endedAt := now.Add(30 * time.Second)
	exitCode := 0
	step1.EndedAt = &endedAt
	step1.Status = domain.StepStatusSucceeded
	step1.ExitCode = &exitCode
	if err := s.UpdateStep(ctx, step1); err != nil {
		t.Fatal(err)
	}

	steps, _ = s.ListSteps(ctx, "exec-steps")
	if steps[0].Status != domain.StepStatusSucceeded || steps[0].EndedAt == nil {
		t.Fatalf("step update not reflected: %+v", steps[0])
	}
}
