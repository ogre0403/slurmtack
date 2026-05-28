package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/evidence"
	"github.com/slurmtack/slurmtack/internal/store"
)

type DryRunConfig struct {
	LogDir    string
	Direction domain.SwitchDirection
	NodeName  string
}

type DryRunner struct {
	store    store.Store
	runner   *Runner
	evidence *evidence.Writer
	logger   *slog.Logger
}

func NewDryRunner(s store.Store, r *Runner, ew *evidence.Writer, logger *slog.Logger) *DryRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &DryRunner{store: s, runner: r, evidence: ew, logger: logger}
}

func (d *DryRunner) Execute(ctx context.Context, cfg DryRunConfig) (string, error) {
	exec := &domain.Execution{
		ID:            fmt.Sprintf("dryrun-%d", time.Now().UnixNano()),
		NodeName:      cfg.NodeName,
		Direction:     cfg.Direction,
		RequestedBy:   "dry-run",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateRequested,
		StateVersion:  0,
		OverallStatus: domain.OverallStatusActive,
		LogRoot:       d.evidence.ExecutionDir(cfg.NodeName, ""),
	}

	if cfg.Direction == domain.DirectionSlurmToOpenStack {
		exec.DesiredOwner = domain.OwnerOpenStack
		exec.PreviousOwner = domain.OwnerSlurm
	} else {
		exec.DesiredOwner = domain.OwnerSlurm
		exec.PreviousOwner = domain.OwnerOpenStack
	}

	if err := d.store.CreateExecution(ctx, exec); err != nil {
		return "", fmt.Errorf("creating dry-run execution: %w", err)
	}

	exec.LogRoot = d.evidence.ExecutionDir(cfg.NodeName, exec.ID)
	d.store.UpdateExecution(ctx, exec)

	if err := d.evidence.InitExecution(exec); err != nil {
		d.logger.Warn("dry_run.evidence_init_failed", "error", err)
	}

	transitions := d.transitionsForDirection(cfg.Direction)

	for _, state := range transitions {
		d.logger.Info("dry_run.transition", "from_state", exec.CurrentState, "to_state", state)

		d.evidence.AppendEvent(cfg.NodeName, exec.ID, map[string]any{
			"type":       "dry_run_transition",
			"from_state": string(exec.CurrentState),
			"to_state":   string(state),
		})

		if err := d.runner.Transition(ctx, exec.ID, state); err != nil {
			d.logger.Error("dry_run.transition_failed", "to_state", state, "error", err)
			return exec.ID, fmt.Errorf("dry-run transition to %s: %w", state, err)
		}

		exec, _ = d.store.GetExecution(ctx, exec.ID)
	}

	d.evidence.WriteManifestUpdate(exec)
	d.logger.Info("dry_run.completed", "execution_id", exec.ID, "node_name", cfg.NodeName)

	return exec.ID, nil
}

func (d *DryRunner) transitionsForDirection(dir domain.SwitchDirection) []domain.SwitchState {
	if dir == domain.DirectionSlurmToOpenStack {
		return []domain.SwitchState{
			domain.StateAwaitingSourceAllocation,
			domain.StateNodeIdentified,
			domain.StateLocked,
			domain.StatePrecheckPassed,
			domain.StateSourceQuiescing,
			domain.StateSourceDetached,
			domain.StateHostReconfiguring,
			domain.StateRebooting,
			domain.StateHostReachable,
			domain.StateTargetAttaching,
			domain.StateVerifying,
			domain.StateCompleted,
		}
	}
	return []domain.SwitchState{
		domain.StateLocked,
		domain.StatePrecheckPassed,
		domain.StateSourceQuiescing,
		domain.StateSourceDetached,
		domain.StateHostReconfiguring,
		domain.StateRebooting,
		domain.StateHostReachable,
		domain.StateTargetAttaching,
		domain.StateVerifying,
		domain.StateCompleted,
	}
}
