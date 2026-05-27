package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/openstack"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

type fakeOpenStackClient struct {
	computeService      *openstack.ComputeServiceStatus
	computeServiceErr   error
	instances           []openstack.Instance
	instancesErr        error
	activeMigrations    []string
	activeMigrationsErr error
	getComputeCalls     int
	listInstancesCalls  int
	listMigrationsCalls int
	disableComputeCalls int
	enableComputeCalls  int
}

func (f *fakeOpenStackClient) ListInstances(ctx context.Context, hostName string) ([]openstack.Instance, error) {
	f.listInstancesCalls++
	return f.instances, f.instancesErr
}

func (f *fakeOpenStackClient) ListActiveMigrations(ctx context.Context, hostName string) ([]string, error) {
	f.listMigrationsCalls++
	return f.activeMigrations, f.activeMigrationsErr
}

func (f *fakeOpenStackClient) GetComputeService(ctx context.Context, hostName string) (*openstack.ComputeServiceStatus, error) {
	f.getComputeCalls++
	if f.computeServiceErr != nil {
		return nil, f.computeServiceErr
	}
	return f.computeService, nil
}

func (f *fakeOpenStackClient) DisableComputeService(ctx context.Context, hostName, reason string) error {
	f.disableComputeCalls++
	return nil
}

func (f *fakeOpenStackClient) EnableComputeService(ctx context.Context, hostName string) error {
	f.enableComputeCalls++
	return nil
}

func newSourceQuiescingExecution() *domain.Execution {
	return &domain.Execution{
		ID:            "o2s-source-quiesce",
		NodeName:      "gpu-node-01",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateSourceQuiescing,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
		StateVersion:  5,
		OverallStatus: domain.OverallStatusActive,
	}
}

func newSourceQuiesceOrchestrator(t *testing.T, client openstack.Client, logger *slog.Logger) (*Orchestrator, store.Store, *domain.Execution) {
	t.Helper()

	s := store.NewMemoryStore()
	exec := newSourceQuiescingExecution()
	ctx := context.Background()
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	runner := engine.NewRunner(s, logger)
	orch := New(s, runner, nil, nil, client, Config{
		TickInterval:    50 * time.Millisecond,
		SSHPollInterval: 1 * time.Second,
		SSHPollTimeout:  5 * time.Second,
	}, logger)

	return orch, s, exec
}

func TestProcessExecutionKeepsO2SInSourceQuiescingWhileStillDraining(t *testing.T) {
	logger, captured := newCaptureLogger()
	client := &fakeOpenStackClient{
		computeService: &openstack.ComputeServiceStatus{Host: "gpu-node-01", Status: "enabled", State: "up", Enabled: true},
		instances:      []openstack.Instance{{ID: "vm-1", Name: "instance-1", Status: "ACTIVE"}},
	}
	orch, s, exec := newSourceQuiesceOrchestrator(t, client, logger)

	orch.processExecution(context.Background(), exec)

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateSourceQuiescing {
		t.Fatalf("expected state %s, got %s", domain.StateSourceQuiescing, updated.CurrentState)
	}
	if updated.OverallStatus != domain.OverallStatusActive {
		t.Fatalf("expected active status, got %s", updated.OverallStatus)
	}
	if client.getComputeCalls != 1 || client.listInstancesCalls != 1 || client.listMigrationsCalls != 1 {
		t.Fatalf("expected quiesce verification reads once, got compute=%d instances=%d migrations=%d", client.getComputeCalls, client.listInstancesCalls, client.listMigrationsCalls)
	}

	selected := captured.find(trace.EventActionSelected)
	if selected == nil || selected.Attrs["action"] != "verify_source_quiesce" {
		t.Fatalf("expected action.selected for verify_source_quiesce, got %#v", selected)
	}
	progress := captured.find(trace.EventWaitProgress)
	if progress == nil {
		t.Fatal("expected wait.progress log")
	}
	if progress.Attrs["wait_for"] != "openstack_source_quiesce" {
		t.Fatalf("wait.progress wait_for = %q, want %q", progress.Attrs["wait_for"], "openstack_source_quiesce")
	}
	if progress.Attrs["resident_instances"] != "1" {
		t.Fatalf("wait.progress resident_instances = %q, want %q", progress.Attrs["resident_instances"], "1")
	}
	if captured.find(trace.EventActionFailed) != nil {
		t.Fatal("did not expect action.failed log while still draining")
	}
}

func TestProcessExecutionAdvancesO2SToSourceDetachedWhenQuiesced(t *testing.T) {
	logger, captured := newCaptureLogger()
	client := &fakeOpenStackClient{
		computeService: &openstack.ComputeServiceStatus{Host: "gpu-node-01", Status: "disabled", State: "up", Enabled: false},
	}
	orch, s, exec := newSourceQuiesceOrchestrator(t, client, logger)

	orch.processExecution(context.Background(), exec)

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateSourceDetached {
		t.Fatalf("expected state %s, got %s", domain.StateSourceDetached, updated.CurrentState)
	}
	if updated.OverallStatus != domain.OverallStatusActive {
		t.Fatalf("expected active status, got %s", updated.OverallStatus)
	}

	satisfied := captured.find(trace.EventWaitSatisfied)
	if satisfied == nil {
		t.Fatal("expected wait.satisfied log")
	}
	if satisfied.Attrs["action"] != "verify_source_quiesce" {
		t.Fatalf("wait.satisfied action = %q, want %q", satisfied.Attrs["action"], "verify_source_quiesce")
	}
}

func TestProcessExecutionFailsO2SWhenSourceQuiesceCheckErrors(t *testing.T) {
	logger, captured := newCaptureLogger()
	client := &fakeOpenStackClient{
		computeServiceErr: errors.New("nova unavailable"),
	}
	orch, s, exec := newSourceQuiesceOrchestrator(t, client, logger)

	orch.processExecution(context.Background(), exec)

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateFailedNonDestructive {
		t.Fatalf("expected state %s, got %s", domain.StateFailedNonDestructive, updated.CurrentState)
	}
	if updated.OverallStatus != domain.OverallStatusFailed {
		t.Fatalf("expected failed status, got %s", updated.OverallStatus)
	}
	if updated.FinalErrorSummary != "nova unavailable" {
		t.Fatalf("expected final error summary %q, got %q", "nova unavailable", updated.FinalErrorSummary)
	}

	failed := captured.find(trace.EventActionFailed)
	if failed == nil {
		t.Fatal("expected action.failed log")
	}
	if failed.Attrs["action"] != "verify_source_quiesce" {
		t.Fatalf("action.failed action = %q, want %q", failed.Attrs["action"], "verify_source_quiesce")
	}
	execFailed := captured.find(trace.EventExecutionFailed)
	if execFailed == nil {
		t.Fatal("expected execution.failed log")
	}
	if execFailed.Attrs["terminal_state"] != string(domain.StateFailedNonDestructive) {
		t.Fatalf("execution.failed terminal_state = %q, want %q", execFailed.Attrs["terminal_state"], domain.StateFailedNonDestructive)
	}
}
