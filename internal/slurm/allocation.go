package slurm

import (
	"context"
	"fmt"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/store"
)

type AllocationEvent struct {
	ExecutionID  string
	JobID        string
	NodeName     string
	StateVersion int64
}

type AllocationHandler struct {
	store  store.Store
	runner *engine.Runner
	client Client
}

func NewAllocationHandler(s store.Store, r *engine.Runner, c Client) *AllocationHandler {
	return &AllocationHandler{store: s, runner: r, client: c}
}

func (h *AllocationHandler) SubmitPlaceholder(ctx context.Context, executionID string) error {
	exec, err := h.store.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("getting execution: %w", err)
	}

	if exec.Direction != domain.DirectionSlurmToOpenStack {
		return fmt.Errorf("placeholder job only applies to slurm-to-openstack direction")
	}

	result, err := h.client.SubmitPlaceholderJob(ctx, PlaceholderJobRequest{
		ExecutionID: executionID,
		Constraint:  exec.RequestedSlurmConstraint,
	})
	if err != nil {
		return fmt.Errorf("submitting placeholder job: %w", err)
	}

	exec.PlaceholderJobID = result.JobID
	if err := h.store.UpdateExecution(ctx, exec); err != nil {
		return fmt.Errorf("recording placeholder job id: %w", err)
	}

	if err := h.runner.Transition(ctx, executionID, domain.StateAwaitingSourceAllocation); err != nil {
		return fmt.Errorf("transitioning to awaiting_source_allocation: %w", err)
	}

	return nil
}

func (h *AllocationHandler) HandleAllocationEvent(ctx context.Context, event AllocationEvent) error {
	exec, err := h.store.GetExecution(ctx, event.ExecutionID)
	if err != nil {
		return fmt.Errorf("getting execution: %w", err)
	}

	if exec.CurrentState != domain.StateAwaitingSourceAllocation {
		return fmt.Errorf("ignoring allocation event: execution not in awaiting_source_allocation (current: %s)", exec.CurrentState)
	}

	if event.StateVersion != exec.StateVersion {
		return fmt.Errorf("ignoring stale allocation event: version %d != current %d", event.StateVersion, exec.StateVersion)
	}

	now := time.Now()
	exec.NodeName = event.NodeName
	exec.PlaceholderJobID = event.JobID
	exec.AllocationEventAt = &now
	if err := h.store.UpdateExecution(ctx, exec); err != nil {
		return fmt.Errorf("binding node to execution: %w", err)
	}

	if err := h.runner.Transition(ctx, event.ExecutionID, domain.StateNodeIdentified); err != nil {
		return fmt.Errorf("transitioning to node_identified: %w", err)
	}

	return nil
}
