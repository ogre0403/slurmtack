package engine

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/evidence"
	"github.com/slurmtack/slurmtack/internal/store"
)

func TestDryRunSlurmToOpenStack(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()
	r := NewRunner(s, nil)
	tmpDir := t.TempDir()
	ew := evidence.NewWriter(tmpDir)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	dr := NewDryRunner(s, r, ew, logger)
	execID, err := dr.Execute(ctx, DryRunConfig{
		Direction: domain.DirectionSlurmToOpenStack,
		NodeName:  "gpu-sim-01",
	})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	exec, err := s.GetExecution(ctx, execID)
	if err != nil {
		t.Fatalf("getting execution: %v", err)
	}
	if exec.CurrentState != domain.StateCompleted {
		t.Fatalf("expected completed, got %s", exec.CurrentState)
	}
	if exec.OverallStatus != domain.OverallStatusSucceeded {
		t.Fatalf("expected succeeded, got %s", exec.OverallStatus)
	}

	steps, _ := s.ListSteps(ctx, execID)
	_ = steps
}

func TestDryRunOpenStackToSlurm(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()
	r := NewRunner(s, nil)
	tmpDir := t.TempDir()
	ew := evidence.NewWriter(tmpDir)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	dr := NewDryRunner(s, r, ew, logger)
	execID, err := dr.Execute(ctx, DryRunConfig{
		Direction: domain.DirectionOpenStackToSlurm,
		NodeName:  "gpu-sim-02",
	})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	exec, err := s.GetExecution(ctx, execID)
	if err != nil {
		t.Fatalf("getting execution: %v", err)
	}
	if exec.CurrentState != domain.StateCompleted {
		t.Fatalf("expected completed, got %s", exec.CurrentState)
	}
}
