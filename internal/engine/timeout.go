package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
)

type TimeoutConfig struct {
	AllocationWait time.Duration
	DrainWait      time.Duration
	RebootWait     time.Duration
	VerifyWait     time.Duration
	DefaultRetries int
}

func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		AllocationWait: 30 * time.Minute,
		DrainWait:      15 * time.Minute,
		RebootWait:     10 * time.Minute,
		VerifyWait:     5 * time.Minute,
		DefaultRetries: 3,
	}
}

type TimeoutChecker struct {
	store  store.Store
	runner *Runner
	config TimeoutConfig
}

func NewTimeoutChecker(s store.Store, r *Runner, cfg TimeoutConfig) *TimeoutChecker {
	return &TimeoutChecker{store: s, runner: r, config: cfg}
}

func (tc *TimeoutChecker) Check(ctx context.Context, executionID string) error {
	exec, err := tc.store.GetExecution(ctx, executionID)
	if err != nil {
		return err
	}

	if exec.CurrentState.IsTerminal() {
		return nil
	}

	steps, err := tc.store.ListSteps(ctx, executionID)
	if err != nil {
		return err
	}

	var lastStepTime time.Time
	for _, s := range steps {
		if s.StartedAt.After(lastStepTime) {
			lastStepTime = s.StartedAt
		}
	}
	if lastStepTime.IsZero() {
		lastStepTime = exec.RequestedAt
	}

	elapsed := time.Since(lastStepTime)
	timeout := tc.timeoutForState(exec.CurrentState)

	if timeout > 0 && elapsed > timeout {
		class := tc.failureClassForTimeout(exec.CurrentState)
		return tc.runner.FailExecution(ctx, executionID, class,
			"timeout",
			fmt.Sprintf("state %s exceeded timeout of %s", exec.CurrentState, timeout),
		)
	}

	return nil
}

func (tc *TimeoutChecker) timeoutForState(state domain.SwitchState) time.Duration {
	switch state {
	case domain.StateAwaitingSourceAllocation:
		return tc.config.AllocationWait
	case domain.StateSourceQuiescing:
		return tc.config.DrainWait
	case domain.StateRebooting:
		return tc.config.RebootWait
	case domain.StateVerifying:
		return tc.config.VerifyWait
	default:
		return 0
	}
}

func (tc *TimeoutChecker) failureClassForTimeout(state domain.SwitchState) domain.FailureClass {
	switch state {
	case domain.StateRebooting:
		return domain.FailureUnknownAfterReboot
	case domain.StateVerifying:
		return domain.FailureVerificationFailed
	case domain.StateAwaitingSourceAllocation, domain.StateSourceQuiescing:
		return domain.FailureTransient
	default:
		return domain.FailureTransient
	}
}

type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

func (tc *TimeoutChecker) ShouldRetry(ctx context.Context, executionID, stepName string) (bool, error) {
	steps, err := tc.store.ListSteps(ctx, executionID)
	if err != nil {
		return false, err
	}

	attempts := 0
	for _, s := range steps {
		if s.StepName == stepName && s.Status == domain.StepStatusFailed {
			attempts++
		}
	}

	return attempts < tc.config.DefaultRetries, nil
}
