package openstack

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

func (h *QuiesceHandler) Name() string { return "openstack_quiesce" }
func (h *QuiesceHandler) Quiesce(ctx context.Context, exec *domain.Execution) error {
	return h.Execute(ctx, exec)
}

func (h *QuiesceHandler) Execute(ctx context.Context, exec *domain.Execution) error {
	if err := h.client.DisableComputeService(ctx, exec.NodeName, fmt.Sprintf("gpu-switch execution %s", exec.ID)); err != nil {
		return fmt.Errorf("disabling compute service: %w", err)
	}
	instances, err := h.client.ListInstances(ctx, exec.NodeName)
	if err != nil {
		return fmt.Errorf("verifying instances cleared: %w", err)
	}
	if len(instances) > 0 {
		return fmt.Errorf("node %s still has %d instances after disable", exec.NodeName, len(instances))
	}
	return nil
}

type DetachHandler struct {
	client Client
}

func NewDetachHandler(c Client) *DetachHandler {
	return &DetachHandler{client: c}
}

func (h *DetachHandler) Name() string { return "openstack_detach" }
func (h *DetachHandler) Detach(ctx context.Context, exec *domain.Execution) error {
	return h.Execute(ctx, exec)
}

func (h *DetachHandler) Execute(ctx context.Context, exec *domain.Execution) error {
	svc, err := h.client.GetComputeService(ctx, exec.NodeName)
	if err != nil {
		return fmt.Errorf("checking compute service: %w", err)
	}
	if svc.Enabled {
		return fmt.Errorf("compute service still enabled on %s", exec.NodeName)
	}
	return nil
}

type AttachHandler struct {
	client Client
}

func NewAttachHandler(c Client) *AttachHandler {
	return &AttachHandler{client: c}
}

func (h *AttachHandler) Name() string { return "openstack_attach" }
func (h *AttachHandler) Attach(ctx context.Context, exec *domain.Execution) error {
	return h.Execute(ctx, exec)
}

func (h *AttachHandler) Execute(ctx context.Context, exec *domain.Execution) error {
	return h.client.EnableComputeService(ctx, exec.NodeName)
}

type VerifyHandler struct {
	client Client
}

func NewVerifyHandler(c Client) *VerifyHandler {
	return &VerifyHandler{client: c}
}

func (h *VerifyHandler) Name() string { return "openstack_verify" }
func (h *VerifyHandler) Verify(ctx context.Context, exec *domain.Execution) error {
	return h.Execute(ctx, exec)
}

func (h *VerifyHandler) Execute(ctx context.Context, exec *domain.Execution) error {
	svc, err := h.client.GetComputeService(ctx, exec.NodeName)
	if err != nil {
		return fmt.Errorf("checking compute service: %w", err)
	}
	if !svc.Enabled {
		return fmt.Errorf("compute service not enabled on %s", exec.NodeName)
	}
	if svc.State != "up" {
		return fmt.Errorf("compute service not up on %s (state: %s)", exec.NodeName, svc.State)
	}
	return nil
}

var (
	_ engine.SourceQuiesceHandler = (*QuiesceHandler)(nil)
	_ engine.SourceDetachHandler  = (*DetachHandler)(nil)
	_ engine.TargetAttachHandler  = (*AttachHandler)(nil)
	_ engine.VerificationHandler  = (*VerifyHandler)(nil)
)
