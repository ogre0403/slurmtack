package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

// attachTargetStep returns the recorded attach_target step for the execution.
func attachTargetStep(t *testing.T, s store.Store, execID string) *domain.StepRecord {
	t.Helper()
	steps, err := s.ListSteps(context.Background(), execID)
	if err != nil {
		t.Fatalf("ListSteps() error = %v", err)
	}
	for _, step := range steps {
		if step.StepName == domain.StepAttachTarget {
			return step
		}
	}
	t.Fatal("expected attach_target step to be recorded")
	return nil
}

// TestSlurmToOpenStackAttachFailurePreservesStepErrorSummary proves that when
// enabling the OpenStack compute service fails, the failed attach_target step
// preserves an error_summary derived from the attach error, and that it matches
// the execution-level terminal failure summary.
func TestSlurmToOpenStackAttachFailurePreservesStepErrorSummary(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()
	exec := newHostReachableExecution(domain.DirectionSlurmToOpenStack)
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution() error = %v", err)
	}

	runner := engine.NewRunner(s, nil)
	osClient := &fakeOpenStackClient{enableComputeErr: errors.New("compute service enable rejected")}
	orch := New(s, runner, nil, nil, osClient, Config{}, nil)

	orch.processExecution(ctx, exec)

	updated, err := s.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}

	const wantSummary = "compute service enable rejected"
	if updated.FinalErrorSummary != wantSummary {
		t.Fatalf("FinalErrorSummary = %q, want %q", updated.FinalErrorSummary, wantSummary)
	}

	step := attachTargetStep(t, s, exec.ID)
	if step.Status != domain.StepStatusFailed {
		t.Fatalf("attach_target step status = %s, want failed", step.Status)
	}
	if step.ErrorSummary != wantSummary {
		t.Fatalf("attach_target step error_summary = %q, want %q", step.ErrorSummary, wantSummary)
	}
	if step.ErrorSummary != updated.FinalErrorSummary {
		t.Fatalf("step error_summary %q should match execution FinalErrorSummary %q", step.ErrorSummary, updated.FinalErrorSummary)
	}
}

// TestOpenStackToSlurmAttachFailurePreservesStepErrorSummary proves that when
// restoring Slurm attachment readiness fails, the failed attach_target step
// preserves an error_summary derived from the attach error, consistent with the
// execution-level terminal failure summary.
func TestOpenStackToSlurmAttachFailurePreservesStepErrorSummary(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()
	exec := newHostReachableExecution(domain.DirectionOpenStackToSlurm)
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution() error = %v", err)
	}

	runner := engine.NewRunner(s, nil)
	sshRunner := &recordingSSHRunner{}
	// An unsupported node state makes EnsureNodeReadyForAttach fail after the
	// slurmd restore commands succeed.
	client := &attachTestSlurmClient{
		nodeState: &slurm.NodeState{NodeName: exec.NodeName, State: "fail"},
	}
	orch := New(s, runner, sshRunner, client, nil, Config{}, nil)

	orch.processExecution(ctx, exec)

	updated, err := s.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}

	const wantSummary = "node gpu-node-01 not attachable (state: fail)"
	if updated.FinalErrorSummary != wantSummary {
		t.Fatalf("FinalErrorSummary = %q, want %q", updated.FinalErrorSummary, wantSummary)
	}

	step := attachTargetStep(t, s, exec.ID)
	if step.Status != domain.StepStatusFailed {
		t.Fatalf("attach_target step status = %s, want failed", step.Status)
	}
	if step.ErrorSummary != wantSummary {
		t.Fatalf("attach_target step error_summary = %q, want %q", step.ErrorSummary, wantSummary)
	}
	if step.ErrorSummary != updated.FinalErrorSummary {
		t.Fatalf("step error_summary %q should match execution FinalErrorSummary %q", step.ErrorSummary, updated.FinalErrorSummary)
	}
}
