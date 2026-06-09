package openstack

import (
	"context"
	"fmt"
	"strings"
)

type ReadinessBlocker struct {
	Kind    string
	Summary string
}

type ReadinessResult struct {
	Ready    bool
	Blockers []ReadinessBlocker
}

func (r *ReadinessResult) ErrorSummary() string {
	if len(r.Blockers) == 0 {
		return ""
	}
	parts := make([]string, len(r.Blockers))
	for i, b := range r.Blockers {
		parts[i] = b.Summary
	}
	return strings.Join(parts, "; ")
}

func EvaluateSourceReadiness(ctx context.Context, client Client, hostName string) (*ReadinessResult, error) {
	_, err := client.GetComputeService(ctx, hostName)
	if err != nil {
		return nil, fmt.Errorf("checking compute service on %s: %w", hostName, err)
	}

	result := &ReadinessResult{Ready: true}

	instances, err := client.ListInstances(ctx, hostName)
	if err != nil {
		return nil, fmt.Errorf("listing instances on %s: %w", hostName, err)
	}
	if len(instances) > 0 {
		result.Ready = false
		result.Blockers = append(result.Blockers, ReadinessBlocker{
			Kind:    "resident_instances",
			Summary: fmt.Sprintf("resident instances: %d", len(instances)),
		})
	}

	migrations, err := client.ListActiveMigrations(ctx, hostName)
	if err != nil {
		return nil, fmt.Errorf("listing migrations on %s: %w", hostName, err)
	}
	if len(migrations) > 0 {
		result.Ready = false
		result.Blockers = append(result.Blockers, ReadinessBlocker{
			Kind:    "active_migrations",
			Summary: fmt.Sprintf("active migrations: %d", len(migrations)),
		})
	}

	return result, nil
}
