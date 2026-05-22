package openstack

import (
	"context"
	"fmt"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
)

type PrecheckHandler struct {
	client Client
}

func NewPrecheckHandler(c Client) *PrecheckHandler {
	return &PrecheckHandler{client: c}
}

func (h *PrecheckHandler) Name() string {
	return "openstack_precheck"
}

func (h *PrecheckHandler) Execute(ctx context.Context, exec *domain.Execution) error {
	if exec.Direction != domain.DirectionOpenStackToSlurm {
		return fmt.Errorf("openstack precheck only applies to openstack-to-slurm direction")
	}

	instances, err := h.client.ListInstances(ctx, exec.NodeName)
	if err != nil {
		return fmt.Errorf("listing instances on %s: %w", exec.NodeName, err)
	}
	if len(instances) > 0 {
		return fmt.Errorf("node %s still has %d resident instances", exec.NodeName, len(instances))
	}

	migrations, err := h.client.ListActiveMigrations(ctx, exec.NodeName)
	if err != nil {
		return fmt.Errorf("listing migrations on %s: %w", exec.NodeName, err)
	}
	if len(migrations) > 0 {
		return fmt.Errorf("node %s has %d in-flight operations: %v", exec.NodeName, len(migrations), migrations)
	}

	svc, err := h.client.GetComputeService(ctx, exec.NodeName)
	if err != nil {
		return fmt.Errorf("checking compute service on %s: %w", exec.NodeName, err)
	}
	if svc.Enabled {
		return fmt.Errorf("compute service on %s is still enabled", exec.NodeName)
	}

	return nil
}

var _ engine.StepHandler = (*PrecheckHandler)(nil)
