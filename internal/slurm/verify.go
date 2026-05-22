package slurm

import (
	"context"
	"fmt"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
)

type AttachHandler struct {
	client Client
}

func NewAttachHandler(c Client) *AttachHandler {
	return &AttachHandler{client: c}
}

func (h *AttachHandler) Name() string { return "slurm_attach" }
func (h *AttachHandler) Attach(ctx context.Context, exec *domain.Execution) error {
	return h.Execute(ctx, exec)
}

func (h *AttachHandler) Execute(ctx context.Context, exec *domain.Execution) error {
	return h.client.ResumeNode(ctx, exec.NodeName)
}

type VerifyHandler struct {
	client Client
}

func NewVerifyHandler(c Client) *VerifyHandler {
	return &VerifyHandler{client: c}
}

func (h *VerifyHandler) Name() string { return "slurm_verify" }
func (h *VerifyHandler) Verify(ctx context.Context, exec *domain.Execution) error {
	return h.Execute(ctx, exec)
}

func (h *VerifyHandler) Execute(ctx context.Context, exec *domain.Execution) error {
	state, err := h.client.GetNodeState(ctx, exec.NodeName)
	if err != nil {
		return fmt.Errorf("getting node state: %w", err)
	}
	if state.State != "idle" && state.State != "alloc" && state.State != "mixed" {
		return fmt.Errorf("node %s not schedulable (state: %s)", exec.NodeName, state.State)
	}
	if len(state.GRES) == 0 {
		return fmt.Errorf("node %s reports no GRES", exec.NodeName)
	}
	return nil
}

var (
	_ engine.TargetAttachHandler = (*AttachHandler)(nil)
	_ engine.VerificationHandler = (*VerifyHandler)(nil)
)
