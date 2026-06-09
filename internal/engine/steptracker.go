package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

type StepTracker struct {
	store  store.Store
	logger *slog.Logger
}

func NewStepTracker(s store.Store, logger *slog.Logger) *StepTracker {
	return &StepTracker{store: s, logger: trace.OrDefault(logger)}
}

func (t *StepTracker) StartStep(ctx context.Context, executionID, stepName, host string) (*domain.StepRecord, error) {
	steps, err := t.store.ListSteps(ctx, executionID)
	if err != nil {
		return nil, fmt.Errorf("listing steps: %w", err)
	}

	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].StepName == stepName && steps[i].Status == domain.StepStatusRunning {
			return steps[i], nil
		}
	}

	step := &domain.StepRecord{
		ExecutionID: executionID,
		StepName:    stepName,
		Sequence:    len(steps) + 1,
		Host:        host,
		StartedAt:   time.Now(),
		Status:      domain.StepStatusRunning,
	}

	if err := t.store.CreateStep(ctx, step); err != nil {
		return nil, fmt.Errorf("creating step: %w", err)
	}

	t.logger.Info(trace.EventStepStarted,
		"execution_id", executionID,
		"step_name", stepName,
		"sequence", step.Sequence,
	)

	return step, nil
}

func (t *StepTracker) FinishStep(ctx context.Context, step *domain.StepRecord, status domain.StepStatus, opts ...StepOption) error {
	now := time.Now()
	step.EndedAt = &now
	step.Status = status

	for _, opt := range opts {
		opt(step)
	}

	if err := t.store.UpdateStep(ctx, step); err != nil {
		return fmt.Errorf("updating step: %w", err)
	}

	event := trace.EventStepSucceeded
	if status == domain.StepStatusFailed {
		event = trace.EventStepFailed
	}

	t.logger.Info(event,
		"execution_id", step.ExecutionID,
		"step_name", step.StepName,
		"sequence", step.Sequence,
		"status", string(status),
	)

	return nil
}

func (t *StepTracker) CloseRunningStep(ctx context.Context, executionID string, status domain.StepStatus, opts ...StepOption) error {
	steps, err := t.store.ListSteps(ctx, executionID)
	if err != nil {
		return fmt.Errorf("listing steps: %w", err)
	}

	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Status == domain.StepStatusRunning {
			return t.FinishStep(ctx, steps[i], status, opts...)
		}
	}

	return nil
}

type StepOption func(*domain.StepRecord)

func WithExitCode(code int) StepOption {
	return func(s *domain.StepRecord) {
		s.ExitCode = &code
	}
}

func WithErrorClass(class domain.FailureClass) StepOption {
	return func(s *domain.StepRecord) {
		s.ErrorClass = class
	}
}

func WithRetryCount(count int) StepOption {
	return func(s *domain.StepRecord) {
		s.RetryCount = count
	}
}

func WithCommandID(id string) StepOption {
	return func(s *domain.StepRecord) {
		s.CommandID = id
	}
}

func WithOutputPaths(stdout, stderr string) StepOption {
	return func(s *domain.StepRecord) {
		s.StdoutPath = stdout
		s.StderrPath = stderr
	}
}

func WithSnapshotPaths(before, after string) StepOption {
	return func(s *domain.StepRecord) {
		s.SnapshotBeforePath = before
		s.SnapshotAfterPath = after
	}
}

func WithErrorSummary(summary string) StepOption {
	return func(s *domain.StepRecord) {
		s.ErrorSummary = summary
	}
}
