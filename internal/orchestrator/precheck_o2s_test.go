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
)

func newLockedO2SExecution() *domain.Execution {
	return &domain.Execution{
		ID:            "o2s-precheck-test",
		NodeName:      "gpu-node-01",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateLocked,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
		StateVersion:  3,
		OverallStatus: domain.OverallStatusActive,
	}
}

func newPrecheckOrchestrator(t *testing.T, client openstack.Client, logger *slog.Logger) (*Orchestrator, store.Store, *domain.Execution) {
	t.Helper()

	s := store.NewMemoryStore()
	exec := newLockedO2SExecution()
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

func TestO2SPrecheckBlockedByResidentInstances(t *testing.T) {
	logger, _ := newCaptureLogger()
	client := &fakeOpenStackClient{
		computeService: &openstack.ComputeServiceStatus{Host: "gpu-node-01", Status: "disabled", State: "up", Enabled: false},
		instances:      []openstack.Instance{{ID: "vm-1", Name: "instance-1", Status: "ACTIVE"}, {ID: "vm-2", Name: "instance-2", Status: "ACTIVE"}},
	}
	orch, s, exec := newPrecheckOrchestrator(t, client, logger)

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
	if updated.FinalErrorCode != "precheck_blocked" {
		t.Fatalf("expected error code precheck_blocked, got %q", updated.FinalErrorCode)
	}
	if updated.FinalErrorSummary != "resident instances: 2" {
		t.Fatalf("expected error summary %q, got %q", "resident instances: 2", updated.FinalErrorSummary)
	}

	steps, _ := s.ListSteps(context.Background(), exec.ID)
	var precheckStep *domain.StepRecord
	for _, step := range steps {
		if step.StepName == domain.StepPrecheck {
			precheckStep = step
		}
	}
	if precheckStep == nil {
		t.Fatal("expected precheck step to be recorded")
	}
	if precheckStep.Status != domain.StepStatusFailed {
		t.Fatalf("expected precheck step status failed, got %s", precheckStep.Status)
	}
	if precheckStep.ErrorClass != domain.FailurePrecheckBlocked {
		t.Fatalf("expected precheck step error_class precheck_blocked, got %s", precheckStep.ErrorClass)
	}
	if precheckStep.ErrorSummary != "resident instances: 2" {
		t.Fatalf("expected precheck step error_summary %q, got %q", "resident instances: 2", precheckStep.ErrorSummary)
	}
}

func TestO2SPrecheckBlockedByActiveMigrations(t *testing.T) {
	logger, _ := newCaptureLogger()
	client := &fakeOpenStackClient{
		computeService:   &openstack.ComputeServiceStatus{Host: "gpu-node-01", Status: "disabled", State: "up", Enabled: false},
		activeMigrations: []string{"migration-1"},
	}
	orch, s, exec := newPrecheckOrchestrator(t, client, logger)

	orch.processExecution(context.Background(), exec)

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateFailedNonDestructive {
		t.Fatalf("expected state %s, got %s", domain.StateFailedNonDestructive, updated.CurrentState)
	}
	if updated.FinalErrorCode != "precheck_blocked" {
		t.Fatalf("expected error code precheck_blocked, got %q", updated.FinalErrorCode)
	}
	if updated.FinalErrorSummary != "active migrations: 1" {
		t.Fatalf("expected error summary %q, got %q", "active migrations: 1", updated.FinalErrorSummary)
	}
}

func TestO2SPrecheckBlockedByMultipleBlockers(t *testing.T) {
	logger, _ := newCaptureLogger()
	client := &fakeOpenStackClient{
		computeService:   &openstack.ComputeServiceStatus{Host: "gpu-node-01", Status: "disabled", State: "up", Enabled: false},
		instances:        []openstack.Instance{{ID: "vm-1", Name: "instance-1", Status: "ACTIVE"}},
		activeMigrations: []string{"migration-1", "migration-2"},
	}
	orch, s, exec := newPrecheckOrchestrator(t, client, logger)

	orch.processExecution(context.Background(), exec)

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateFailedNonDestructive {
		t.Fatalf("expected state %s, got %s", domain.StateFailedNonDestructive, updated.CurrentState)
	}
	expectedSummary := "resident instances: 1; active migrations: 2"
	if updated.FinalErrorSummary != expectedSummary {
		t.Fatalf("expected error summary %q, got %q", expectedSummary, updated.FinalErrorSummary)
	}

	steps, _ := s.ListSteps(context.Background(), exec.ID)
	var precheckStep *domain.StepRecord
	for _, step := range steps {
		if step.StepName == domain.StepPrecheck {
			precheckStep = step
		}
	}
	if precheckStep == nil {
		t.Fatal("expected precheck step to be recorded")
	}
	if precheckStep.ErrorSummary != expectedSummary {
		t.Fatalf("expected precheck step error_summary %q, got %q", expectedSummary, precheckStep.ErrorSummary)
	}
}

func TestO2SPrecheckPassesWhenSourceIsReady(t *testing.T) {
	logger, _ := newCaptureLogger()
	client := &fakeOpenStackClient{
		computeService: &openstack.ComputeServiceStatus{Host: "gpu-node-01", Status: "disabled", State: "up", Enabled: false},
	}
	orch, s, exec := newPrecheckOrchestrator(t, client, logger)

	orch.processExecution(context.Background(), exec)

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StatePrecheckPassed {
		t.Fatalf("expected state %s, got %s", domain.StatePrecheckPassed, updated.CurrentState)
	}
	if updated.OverallStatus != domain.OverallStatusActive {
		t.Fatalf("expected active status, got %s", updated.OverallStatus)
	}

	steps, _ := s.ListSteps(context.Background(), exec.ID)
	var precheckStep *domain.StepRecord
	for _, step := range steps {
		if step.StepName == domain.StepPrecheck {
			precheckStep = step
		}
	}
	if precheckStep == nil {
		t.Fatal("expected precheck step to be recorded")
	}
	if precheckStep.Status != domain.StepStatusSucceeded {
		t.Fatalf("expected precheck step status succeeded, got %s", precheckStep.Status)
	}
}

func TestO2SPrecheckBlockedByReadinessCheckError(t *testing.T) {
	logger, _ := newCaptureLogger()
	client := &fakeOpenStackClient{
		computeServiceErr: errors.New("connection refused"),
	}
	orch, s, exec := newPrecheckOrchestrator(t, client, logger)

	orch.processExecution(context.Background(), exec)

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateFailedNonDestructive {
		t.Fatalf("expected state %s, got %s", domain.StateFailedNonDestructive, updated.CurrentState)
	}
	if updated.FinalErrorCode != "precheck_blocked" {
		t.Fatalf("expected error code precheck_blocked, got %q", updated.FinalErrorCode)
	}
	if updated.FinalErrorSummary != "source readiness check failed: connection refused" {
		t.Fatalf("expected error summary %q, got %q", "source readiness check failed: connection refused", updated.FinalErrorSummary)
	}

	steps, _ := s.ListSteps(context.Background(), exec.ID)
	var precheckStep *domain.StepRecord
	for _, step := range steps {
		if step.StepName == domain.StepPrecheck {
			precheckStep = step
		}
	}
	if precheckStep == nil {
		t.Fatal("expected precheck step to be recorded")
	}
	if precheckStep.Status != domain.StepStatusFailed {
		t.Fatalf("expected precheck step status failed, got %s", precheckStep.Status)
	}
	if precheckStep.ErrorClass != domain.FailurePrecheckBlocked {
		t.Fatalf("expected precheck step error_class precheck_blocked, got %s", precheckStep.ErrorClass)
	}
	if precheckStep.ErrorSummary != "source readiness check failed: connection refused" {
		t.Fatalf("expected precheck step error_summary %q, got %q", "source readiness check failed: connection refused", precheckStep.ErrorSummary)
	}
}

func TestO2SPrecheckDoesNotReachSourceQuiescing(t *testing.T) {
	logger, _ := newCaptureLogger()
	client := &fakeOpenStackClient{
		computeService:   &openstack.ComputeServiceStatus{Host: "gpu-node-01", Status: "disabled", State: "up", Enabled: false},
		instances:        []openstack.Instance{{ID: "vm-1", Name: "instance-1", Status: "ACTIVE"}},
		activeMigrations: []string{"migration-1"},
	}
	orch, s, exec := newPrecheckOrchestrator(t, client, logger)

	orch.processExecution(context.Background(), exec)

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState == domain.StatePrecheckPassed || updated.CurrentState == domain.StateSourceQuiescing {
		t.Fatalf("execution should NOT have reached %s, got %s", updated.CurrentState, updated.CurrentState)
	}
	if updated.CurrentState != domain.StateFailedNonDestructive {
		t.Fatalf("expected state %s, got %s", domain.StateFailedNonDestructive, updated.CurrentState)
	}
}
