package engine

import (
	"context"
	"fmt"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
)

type Event struct {
	Type         string
	ExecutionID  string
	StateVersion int64
	NodeName     string
	Payload      map[string]string
}

type EventHandler struct {
	store  store.Store
	runner *Runner
}

func NewEventHandler(s store.Store, r *Runner) *EventHandler {
	return &EventHandler{store: s, runner: r}
}

func (h *EventHandler) Handle(ctx context.Context, event Event) error {
	exec, err := h.store.GetExecution(ctx, event.ExecutionID)
	if err != nil {
		return fmt.Errorf("event for unknown execution %s: %w", event.ExecutionID, err)
	}

	if exec.StateVersion != event.StateVersion {
		return fmt.Errorf("stale event: version %d does not match current %d", event.StateVersion, exec.StateVersion)
	}

	if exec.CurrentState.IsTerminal() {
		return fmt.Errorf("ignoring event for terminal execution %s (state: %s)", event.ExecutionID, exec.CurrentState)
	}

	switch event.Type {
	case "allocation":
		return h.handleAllocation(ctx, exec, event)
	case "drained":
		return h.handleDrained(ctx, exec, event)
	case "host_reachable":
		return h.handleHostReachable(ctx, exec, event)
	default:
		return fmt.Errorf("unknown event type: %s", event.Type)
	}
}

func (h *EventHandler) handleAllocation(ctx context.Context, exec *domain.Execution, event Event) error {
	if exec.CurrentState != domain.StateAwaitingSourceAllocation {
		return fmt.Errorf("allocation event not expected in state %s", exec.CurrentState)
	}
	return nil
}

func (h *EventHandler) handleDrained(ctx context.Context, exec *domain.Execution, event Event) error {
	if exec.CurrentState != domain.StateSourceQuiescing {
		return fmt.Errorf("drained event not expected in state %s", exec.CurrentState)
	}
	return h.runner.Transition(ctx, exec.ID, domain.StateSourceDetached)
}

func (h *EventHandler) handleHostReachable(ctx context.Context, exec *domain.Execution, event Event) error {
	if exec.CurrentState != domain.StateRebooting {
		return fmt.Errorf("host_reachable event not expected in state %s", exec.CurrentState)
	}
	return h.runner.Transition(ctx, exec.ID, domain.StateHostReachable)
}
