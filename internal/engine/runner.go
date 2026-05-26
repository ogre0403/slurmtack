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

type StepHandler interface {
	Name() string
	Execute(ctx context.Context, exec *domain.Execution) error
}

type Runner struct {
	store  store.Store
	logger *slog.Logger
}

func NewRunner(s store.Store, logger *slog.Logger) *Runner {
	return &Runner{store: s, logger: trace.OrDefault(logger)}
}

func (r *Runner) Transition(ctx context.Context, executionID string, to domain.SwitchState) error {
	exec, err := r.store.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("getting execution: %w", err)
	}

	r.logger.Info(trace.EventTransitionRequested,
		"execution_id", executionID,
		"from_state", string(exec.CurrentState),
		"to_state", string(to),
	)

	if !domain.IsValidTransition(exec.CurrentState, to) {
		r.logger.Warn(trace.EventTransitionFailed,
			"execution_id", executionID,
			"from_state", string(exec.CurrentState),
			"to_state", string(to),
			"reason", "invalid transition",
		)
		return fmt.Errorf("invalid transition from %s to %s", exec.CurrentState, to)
	}

	if err := r.store.AdvanceState(ctx, executionID, exec.StateVersion, to); err != nil {
		r.logger.Warn(trace.EventTransitionFailed,
			"execution_id", executionID,
			"from_state", string(exec.CurrentState),
			"to_state", string(to),
			"error", err.Error(),
		)
		return fmt.Errorf("advancing state: %w", err)
	}

	r.logger.Info(trace.EventTransitionSucceeded,
		"execution_id", executionID,
		"from_state", string(exec.CurrentState),
		"to_state", string(to),
	)
	return nil
}

func (r *Runner) FailExecution(ctx context.Context, executionID string, class domain.FailureClass, errCode, errSummary string) error {
	exec, err := r.store.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("getting execution: %w", err)
	}

	targetState := classifyTerminalState(exec.CurrentState, class)

	if !domain.IsValidTransition(exec.CurrentState, targetState) {
		return fmt.Errorf("cannot fail from %s to %s", exec.CurrentState, targetState)
	}

	if err := r.store.AdvanceState(ctx, executionID, exec.StateVersion, targetState); err != nil {
		return fmt.Errorf("advancing to failed state: %w", err)
	}

	exec, err = r.store.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("re-reading execution: %w", err)
	}
	exec.FinalErrorCode = errCode
	exec.FinalErrorSummary = errSummary
	if err := r.store.UpdateExecution(ctx, exec); err != nil {
		return fmt.Errorf("updating execution error details: %w", err)
	}

	r.logger.Warn(trace.EventExecutionFailed,
		"execution_id", executionID,
		"failure_class", string(class),
		"terminal_state", string(targetState),
		"error_code", errCode,
		"error_summary", errSummary,
	)
	return nil
}

func (r *Runner) RunStep(ctx context.Context, executionID string, handler StepHandler) error {
	exec, err := r.store.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("getting execution: %w", err)
	}

	steps, err := r.store.ListSteps(ctx, executionID)
	if err != nil {
		return fmt.Errorf("listing steps: %w", err)
	}

	now := time.Now()
	step := &domain.StepRecord{
		ExecutionID: executionID,
		StepName:    handler.Name(),
		Sequence:    len(steps) + 1,
		Host:        exec.NodeName,
		StartedAt:   now,
		Status:      domain.StepStatusRunning,
	}

	if err := r.store.CreateStep(ctx, step); err != nil {
		return fmt.Errorf("creating step record: %w", err)
	}

	r.logger.Info(trace.EventStepStarted,
		"execution_id", executionID,
		"step_name", handler.Name(),
		"sequence", step.Sequence,
	)

	stepErr := handler.Execute(ctx, exec)

	ended := time.Now()
	step.EndedAt = &ended
	if stepErr != nil {
		step.Status = domain.StepStatusFailed
		r.logger.Warn(trace.EventStepFailed,
			"execution_id", executionID,
			"step_name", handler.Name(),
			"sequence", step.Sequence,
			"error", stepErr.Error(),
		)
	} else {
		step.Status = domain.StepStatusSucceeded
		r.logger.Info(trace.EventStepSucceeded,
			"execution_id", executionID,
			"step_name", handler.Name(),
			"sequence", step.Sequence,
		)
	}

	if updateErr := r.store.UpdateStep(ctx, step); updateErr != nil {
		return fmt.Errorf("updating step record: %w", updateErr)
	}

	return stepErr
}

func classifyTerminalState(current domain.SwitchState, class domain.FailureClass) domain.SwitchState {
	switch {
	case class == domain.FailureUnknownAfterReboot:
		return domain.StateFailedManualRecovery
	case class == domain.FailureMutationPartial:
		return domain.StateFailedNeedsRollback
	case current == domain.StateRebooting:
		return domain.StateFailedManualRecovery
	case isPreMutation(current):
		return domain.StateFailedNonDestructive
	default:
		return domain.StateFailedNeedsRollback
	}
}

func isPreMutation(state domain.SwitchState) bool {
	switch state {
	case domain.StateRequested,
		domain.StateAwaitingSourceAllocation,
		domain.StateNodeIdentified,
		domain.StateLocked,
		domain.StatePrecheckPassed,
		domain.StateSourceQuiescing:
		return true
	}
	return false
}
