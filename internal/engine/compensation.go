package engine

import (
	"context"
	"fmt"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
)

type CompensationAction interface {
	Name() string
	CanCompensate(exec *domain.Execution) bool
	Compensate(ctx context.Context, exec *domain.Execution) error
}

type CompensationHandler struct {
	store   store.Store
	runner  *Runner
	actions []CompensationAction
}

func NewCompensationHandler(s store.Store, r *Runner, actions []CompensationAction) *CompensationHandler {
	return &CompensationHandler{store: s, runner: r, actions: actions}
}

func (ch *CompensationHandler) HandleFailure(ctx context.Context, executionID string, class domain.FailureClass, errCode, errSummary string) error {
	exec, err := ch.store.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("getting execution: %w", err)
	}

	switch class {
	case domain.FailureTransient, domain.FailurePrecheckBlocked:
		return ch.runner.FailExecution(ctx, executionID, class, errCode, errSummary)

	case domain.FailureUnknownAfterReboot:
		return ch.runner.FailExecution(ctx, executionID, class, errCode, errSummary)

	case domain.FailureMutationPartial, domain.FailureVerificationFailed:
		return ch.attemptCompensation(ctx, exec, class, errCode, errSummary)

	default:
		return ch.runner.FailExecution(ctx, executionID, class, errCode, errSummary)
	}
}

func (ch *CompensationHandler) attemptCompensation(ctx context.Context, exec *domain.Execution, class domain.FailureClass, errCode, errSummary string) error {
	if err := ch.runner.Transition(ctx, exec.ID, domain.StateCompensating); err != nil {
		return ch.runner.FailExecution(ctx, exec.ID, class, errCode, errSummary)
	}

	exec, _ = ch.store.GetExecution(ctx, exec.ID)

	for _, action := range ch.actions {
		if !action.CanCompensate(exec) {
			continue
		}
		if err := action.Compensate(ctx, exec); err != nil {
			return ch.runner.FailExecution(ctx, exec.ID, domain.FailureMutationPartial,
				"compensation_failed",
				fmt.Sprintf("compensation action %s failed: %v", action.Name(), err),
			)
		}
	}

	if err := ch.runner.Transition(ctx, exec.ID, domain.StateCompleted); err != nil {
		return ch.runner.FailExecution(ctx, exec.ID, class, errCode,
			fmt.Sprintf("compensated but cannot mark complete: %s", errSummary))
	}

	return nil
}
