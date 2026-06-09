package store

import (
	"context"
	"database/sql"
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

func TestCreateAndGetExecutionWithRequestedSlurmPartition(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	exec := &domain.Execution{
		ID:                       "exec-partition",
		Direction:                domain.DirectionSlurmToOpenStack,
		RequestedBy:              "operator",
		RequestedAt:              time.Now().Truncate(time.Second),
		CurrentState:             domain.StateRequested,
		DesiredOwner:             domain.OwnerOpenStack,
		PreviousOwner:            domain.OwnerSlurm,
		StateVersion:             0,
		OverallStatus:            domain.OverallStatusActive,
		RequestedSlurmConstraint: "gpu-a100",
		RequestedSlurmPartition:  "gpu-maint",
	}

	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RequestedSlurmConstraint != "gpu-a100" {
		t.Fatalf("RequestedSlurmConstraint = %q, want gpu-a100", got.RequestedSlurmConstraint)
	}
	if got.RequestedSlurmPartition != "gpu-maint" {
		t.Fatalf("RequestedSlurmPartition = %q, want gpu-maint", got.RequestedSlurmPartition)
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
			ID:       "exec-" + name + "-" + time.Now().Format("150405.000"),
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
	exec.RequestedSlurmPartition = "gpu-maint"
	if err := s.UpdateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	got, _ := s.GetExecution(ctx, "exec-upd")
	if got.NodeName != "gpu-05" || got.PlaceholderJobID != "job-123" || got.RequestedSlurmPartition != "gpu-maint" {
		t.Fatalf("update not persisted: %+v", got)
	}
}

func TestNewSQLiteStoreMigratesRequestedSlurmPartitionColumn(t *testing.T) {
	f, err := os.CreateTemp("", "slurmtack-legacy-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	legacyDB, err := sql.Open("sqlite3", f.Name())
	if err != nil {
		t.Fatal(err)
	}
	legacySchema := `CREATE TABLE IF NOT EXISTS executions (
		id TEXT PRIMARY KEY,
		node_name TEXT NOT NULL DEFAULT '',
		direction TEXT NOT NULL,
		requested_by TEXT NOT NULL,
		requested_at DATETIME NOT NULL,
		current_state TEXT NOT NULL,
		desired_owner TEXT NOT NULL,
		previous_owner TEXT NOT NULL,
		state_version INTEGER NOT NULL DEFAULT 0,
		overall_status TEXT NOT NULL DEFAULT 'active',
		lock_acquired_at DATETIME,
		lock_released_at DATETIME,
		final_error_code TEXT NOT NULL DEFAULT '',
		final_error_summary TEXT NOT NULL DEFAULT '',
		log_root TEXT NOT NULL DEFAULT '',
		placeholder_job_id TEXT NOT NULL DEFAULT '',
		requested_slurm_constraint TEXT NOT NULL DEFAULT '',
		allocation_event_at DATETIME
	)`
	if _, err := legacyDB.Exec(legacySchema); err != nil {
		legacyDB.Close()
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Second)
	if _, err := legacyDB.Exec(`INSERT INTO executions (
		id, node_name, direction, requested_by, requested_at,
		current_state, desired_owner, previous_owner, state_version,
		overall_status, placeholder_job_id, requested_slurm_constraint
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"legacy-exec", "gpu-01", string(domain.DirectionSlurmToOpenStack), "operator", now,
		string(domain.StateRequested), string(domain.OwnerOpenStack), string(domain.OwnerSlurm), 0,
		string(domain.OverallStatusActive), "job-legacy", "gpu-a100",
	); err != nil {
		legacyDB.Close()
		t.Fatal(err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := NewSQLiteStore(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	got, err := s.GetExecution(context.Background(), "legacy-exec")
	if err != nil {
		t.Fatal(err)
	}
	if got.RequestedSlurmPartition != "" {
		t.Fatalf("RequestedSlurmPartition = %q, want empty string after migration", got.RequestedSlurmPartition)
	}

	got.RequestedSlurmPartition = "gpu-maint"
	if err := s.UpdateExecution(context.Background(), got); err != nil {
		t.Fatal(err)
	}

	updated, err := s.GetExecution(context.Background(), "legacy-exec")
	if err != nil {
		t.Fatal(err)
	}
	if updated.RequestedSlurmPartition != "gpu-maint" {
		t.Fatalf("RequestedSlurmPartition = %q, want gpu-maint", updated.RequestedSlurmPartition)
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

func TestStepErrorSummaryRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	exec := &domain.Execution{
		ID: "exec-summary", Direction: domain.DirectionOpenStackToSlurm,
		RequestedBy: "test", RequestedAt: time.Now().Truncate(time.Second),
		CurrentState: domain.StateFailedNonDestructive,
		DesiredOwner: domain.OwnerSlurm, PreviousOwner: domain.OwnerOpenStack,
		OverallStatus: domain.OverallStatusFailed,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatal(err)
	}

	now := time.Now().Truncate(time.Second)
	ended := now.Add(time.Second)
	step := &domain.StepRecord{
		ExecutionID:  "exec-summary",
		StepName:     domain.StepPrecheck,
		Sequence:     1,
		Host:         "gpu-01",
		StartedAt:    now,
		EndedAt:      &ended,
		Status:       domain.StepStatusFailed,
		ErrorClass:   domain.FailurePrecheckBlocked,
		ErrorSummary: "resident instances: 2; active migrations: 1",
	}
	if err := s.CreateStep(ctx, step); err != nil {
		t.Fatalf("create step: %v", err)
	}

	steps, err := s.ListSteps(ctx, "exec-summary")
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].ErrorSummary != "resident instances: 2; active migrations: 1" {
		t.Errorf("ErrorSummary = %q, want %q", steps[0].ErrorSummary, "resident instances: 2; active migrations: 1")
	}
	if steps[0].ErrorClass != domain.FailurePrecheckBlocked {
		t.Errorf("ErrorClass = %q, want %q", steps[0].ErrorClass, domain.FailurePrecheckBlocked)
	}

	// Update via UpdateStep and verify round-trip
	step.ErrorSummary = "resident instances: 5"
	if err := s.UpdateStep(ctx, step); err != nil {
		t.Fatalf("update step: %v", err)
	}
	steps, _ = s.ListSteps(ctx, "exec-summary")
	if steps[0].ErrorSummary != "resident instances: 5" {
		t.Errorf("after update, ErrorSummary = %q, want %q", steps[0].ErrorSummary, "resident instances: 5")
	}
}

func TestStepErrorSummaryMigration(t *testing.T) {
	f, err := os.CreateTemp("", "slurmtack-migration-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	db, err := sql.Open("sqlite3", f.Name()+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatal(err)
	}

	// Create old schema without error_summary column on steps
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS executions (
		id TEXT PRIMARY KEY, node_name TEXT NOT NULL DEFAULT '', direction TEXT NOT NULL,
		requested_by TEXT NOT NULL, requested_at DATETIME NOT NULL,
		current_state TEXT NOT NULL, desired_owner TEXT NOT NULL, previous_owner TEXT NOT NULL,
		state_version INTEGER NOT NULL DEFAULT 0, overall_status TEXT NOT NULL DEFAULT 'active',
		lock_acquired_at DATETIME, lock_released_at DATETIME,
		final_error_code TEXT NOT NULL DEFAULT '', final_error_summary TEXT NOT NULL DEFAULT '',
		log_root TEXT NOT NULL DEFAULT '', placeholder_job_id TEXT NOT NULL DEFAULT '',
		requested_slurm_constraint TEXT NOT NULL DEFAULT '',
		requested_slurm_partition TEXT NOT NULL DEFAULT '',
		requested_slurm_account TEXT NOT NULL DEFAULT '',
		slurm_workload_user TEXT NOT NULL DEFAULT '',
		slurm_workload_token TEXT NOT NULL DEFAULT '',
		placeholder_sif_file TEXT NOT NULL DEFAULT '',
		allocation_event_at DATETIME,
		cancellation_source_state TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS steps (
		execution_id TEXT NOT NULL, step_name TEXT NOT NULL, sequence INTEGER NOT NULL,
		host TEXT NOT NULL DEFAULT '', started_at DATETIME NOT NULL, ended_at DATETIME,
		status TEXT NOT NULL DEFAULT 'pending', retry_count INTEGER NOT NULL DEFAULT 0,
		exit_code INTEGER, error_class TEXT NOT NULL DEFAULT '',
		command_id TEXT NOT NULL DEFAULT '', stdout_path TEXT NOT NULL DEFAULT '',
		stderr_path TEXT NOT NULL DEFAULT '', snapshot_before_path TEXT NOT NULL DEFAULT '',
		snapshot_after_path TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (execution_id, step_name, sequence)
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS leases (
		node_name TEXT PRIMARY KEY, execution_id TEXT NOT NULL, holder TEXT NOT NULL DEFAULT '',
		expires_at DATETIME NOT NULL, state_version INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Opening with NewSQLiteStore should migrate the schema
	s, err := NewSQLiteStore(f.Name())
	if err != nil {
		t.Fatalf("NewSQLiteStore on old schema: %v", err)
	}
	defer s.Close()

	// Verify we can write and read error_summary after migration
	ctx := context.Background()
	exec := &domain.Execution{
		ID: "migrated-exec", Direction: domain.DirectionOpenStackToSlurm,
		RequestedBy: "test", RequestedAt: time.Now().Truncate(time.Second),
		CurrentState: domain.StateLocked, DesiredOwner: domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack, OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution after migration: %v", err)
	}

	step := &domain.StepRecord{
		ExecutionID:  "migrated-exec",
		StepName:     domain.StepPrecheck,
		Sequence:     1,
		Host:         "gpu-01",
		StartedAt:    time.Now().Truncate(time.Second),
		Status:       domain.StepStatusFailed,
		ErrorClass:   domain.FailurePrecheckBlocked,
		ErrorSummary: "active migrations: 3",
	}
	if err := s.CreateStep(ctx, step); err != nil {
		t.Fatalf("create step after migration: %v", err)
	}

	steps, err := s.ListSteps(ctx, "migrated-exec")
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if steps[0].ErrorSummary != "active migrations: 3" {
		t.Errorf("ErrorSummary after migration = %q, want %q", steps[0].ErrorSummary, "active migrations: 3")
	}
}
