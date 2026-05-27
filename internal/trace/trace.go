package trace

import (
	"log/slog"

	"github.com/slurmtack/slurmtack/internal/domain"
)

// Event names emitted across the switch daemon workflow.
const (
	EventRequestAccepted     = "request.accepted"
	EventActionSelected      = "action.selected"
	EventActionStarted       = "action.started"
	EventActionSucceeded     = "action.succeeded"
	EventActionFailed        = "action.failed"
	EventSSHDispatch         = "ssh.dispatch"
	EventTransitionRequested = "transition.requested"
	EventTransitionSucceeded = "transition.succeeded"
	EventTransitionFailed    = "transition.failed"
	EventWaitEntered         = "wait.entered"
	EventWaitProgress        = "wait.progress"
	EventWaitSatisfied       = "wait.satisfied"
	EventWaitTimeout         = "wait.timeout"
	EventStepStarted         = "step.started"
	EventStepSucceeded       = "step.succeeded"
	EventStepFailed          = "step.failed"
	EventExecutionCompleted  = "execution.completed"
	EventExecutionFailed     = "execution.failed"
)

// ForExecution returns a child logger pre-populated with execution-scoped
// fields. node_name is omitted when exec.NodeName is empty (e.g. during
// awaiting_source_allocation before the node is known).
func ForExecution(logger *slog.Logger, exec *domain.Execution) *slog.Logger {
	l := logger.With(
		"execution_id", exec.ID,
		"direction", string(exec.Direction),
		"current_state", string(exec.CurrentState),
		"state_version", exec.StateVersion,
	)
	if exec.NodeName != "" {
		l = l.With("node_name", exec.NodeName)
	}
	return l
}

// OrDefault returns l if non-nil, otherwise slog.Default().
func OrDefault(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.Default()
}
