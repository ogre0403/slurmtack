package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/openstack"
	"github.com/slurmtack/slurmtack/internal/store"
)

func newLockedS2OExecution() *domain.Execution {
	return &domain.Execution{
		ID:            "s2o-precheck-test",
		NodeName:      "gpu-node-01",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateLocked,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  3,
		OverallStatus: domain.OverallStatusActive,
	}
}

func TestS2OPrecheckBlockedByMissingComputeService(t *testing.T) {
	s := store.NewMemoryStore()
	exec := newLockedS2OExecution()
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	client := &fakeOpenStackClient{
		computeServiceErr: errors.New("service not found"),
	}
	runner := engine.NewRunner(s, nil)
	orch := New(s, runner, nil, nil, client, Config{}, nil)

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
	expectedSummary := "compute service on gpu-node-01: service not found"
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
	if precheckStep.Status != domain.StepStatusFailed {
		t.Fatalf("expected precheck step status failed, got %s", precheckStep.Status)
	}
	if precheckStep.ErrorClass != domain.FailurePrecheckBlocked {
		t.Fatalf("expected precheck step error_class precheck_blocked, got %s", precheckStep.ErrorClass)
	}
	if precheckStep.ErrorSummary != expectedSummary {
		t.Fatalf("expected precheck step error_summary %q, got %q", expectedSummary, precheckStep.ErrorSummary)
	}
}

func TestS2OPrecheckPassesWhenComputeServiceExists(t *testing.T) {
	s := store.NewMemoryStore()
	exec := newLockedS2OExecution()
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	client := &fakeOpenStackClient{
		computeService: &openstack.ComputeServiceStatus{Host: "gpu-node-01", Enabled: true},
	}
	runner := engine.NewRunner(s, nil)
	orch := New(s, runner, nil, nil, client, Config{}, nil)

	orch.processExecution(context.Background(), exec)

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StatePrecheckPassed {
		t.Fatalf("expected state %s, got %s", domain.StatePrecheckPassed, updated.CurrentState)
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

func TestS2OPrecheckBlockedByMissingOpenStackClient(t *testing.T) {
	s := store.NewMemoryStore()
	exec := newLockedS2OExecution()
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	runner := engine.NewRunner(s, nil)
	orch := New(s, runner, nil, nil, nil, Config{}, nil)

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
	expectedSummary := "openstack client not configured"
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
	if precheckStep.Status != domain.StepStatusFailed {
		t.Fatalf("expected precheck step status failed, got %s", precheckStep.Status)
	}
	if precheckStep.ErrorClass != domain.FailurePrecheckBlocked {
		t.Fatalf("expected precheck step error_class precheck_blocked, got %s", precheckStep.ErrorClass)
	}
	if precheckStep.ErrorSummary != expectedSummary {
		t.Fatalf("expected precheck step error_summary %q, got %q", expectedSummary, precheckStep.ErrorSummary)
	}
}
