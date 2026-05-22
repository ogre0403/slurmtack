package slurm

import (
	"context"
	"fmt"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
)

type QuiesceHandler struct {
	client Client
}

func NewQuiesceHandler(c Client) *QuiesceHandler {
	return &QuiesceHandler{client: c}
}

func (h *QuiesceHandler) Name() string    { return "slurm_quiesce" }
func (h *QuiesceHandler) Quiesce(ctx context.Context, exec *domain.Execution) error {
	return h.Execute(ctx, exec)
}

func (h *QuiesceHandler) Execute(ctx context.Context, exec *domain.Execution) error {
	return h.client.DrainNode(ctx, exec.NodeName, fmt.Sprintf("gpu-switch execution %s", exec.ID))
}

type DetachHandler struct {
	client Client
}

func NewDetachHandler(c Client) *DetachHandler {
	return &DetachHandler{client: c}
}

func (h *DetachHandler) Name() string { return "slurm_detach" }
func (h *DetachHandler) Detach(ctx context.Context, exec *domain.Execution) error {
	return h.Execute(ctx, exec)
}

func (h *DetachHandler) Execute(ctx context.Context, exec *domain.Execution) error {
	state, err := h.client.GetNodeState(ctx, exec.NodeName)
	if err != nil {
		return fmt.Errorf("checking node state: %w", err)
	}
	if state.State != "drained" && state.State != "down" {
		return fmt.Errorf("node %s not drained (state: %s)", exec.NodeName, state.State)
	}
	return nil
}

var (
	_ engine.SourceQuiesceHandler = (*QuiesceHandler)(nil)
	_ engine.SourceDetachHandler  = (*DetachHandler)(nil)
)
