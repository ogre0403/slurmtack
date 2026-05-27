package orchestrator

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/openstack"
	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

type Config struct {
	TickInterval    time.Duration
	SSHPollInterval time.Duration
	SSHPollTimeout  time.Duration
}

type Orchestrator struct {
	store     store.Store
	runner    *engine.Runner
	sshRunner remote.Runner
	slurm     slurm.Client
	openstack openstack.Client
	cfg       Config
	logger    *slog.Logger
}

func New(s store.Store, runner *engine.Runner, sshRunner remote.Runner, slurmClient slurm.Client, osClient openstack.Client, cfg Config, logger *slog.Logger) *Orchestrator {
	return &Orchestrator{
		store:     s,
		runner:    runner,
		sshRunner: sshRunner,
		slurm:     slurmClient,
		openstack: osClient,
		cfg:       cfg,
		logger:    trace.OrDefault(logger),
	}
}

func (o *Orchestrator) Run(ctx context.Context) {
	ticker := time.NewTicker(o.cfg.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.tick(ctx)
		}
	}
}

func (o *Orchestrator) tick(ctx context.Context) {
	executions, err := o.store.ListActiveExecutions(ctx)
	if err != nil {
		log.Printf("orchestrator: failed to list active executions: %v", err)
		return
	}

	for _, exec := range executions {
		if ctx.Err() != nil {
			return
		}
		o.processExecution(ctx, exec)
	}
}

func actionName(a action) string {
	switch a {
	case actionSubmitPlaceholder:
		return "submit_placeholder"
	case actionAcquireLease:
		return "acquire_lease"
	case actionPrecheck:
		return "precheck"
	case actionAcquireLeaseAndPrecheck:
		return "acquire_lease_and_precheck"
	case actionQuiesce:
		return "quiesce"
	case actionVerifySourceQuiesce:
		return "verify_source_quiesce"
	case actionReconfigure:
		return "reconfigure"
	case actionReboot:
		return "reboot"
	case actionSSHPoll:
		return "ssh_poll"
	case actionAttach:
		return "attach"
	case actionVerify:
		return "verify"
	case actionComplete:
		return "complete"
	}
	return "unknown"
}

func (o *Orchestrator) processExecution(ctx context.Context, exec *domain.Execution) {
	action := o.determineAction(exec)
	if action == actionNone {
		return
	}

	execLog := trace.ForExecution(o.logger, exec)
	execLog.Info(trace.EventActionSelected, "action", actionName(action))

	err := o.executeAction(ctx, exec, action)
	if err == nil {
		return
	}

	if errors.Is(err, store.ErrVersionConflict) {
		execLog.Info("orchestrator.version_conflict", "action", actionName(action))
		return
	}

	execLog.Warn(trace.EventActionFailed, "action", actionName(action), "error", err.Error())
	failClass := classifyFailure(exec.CurrentState)
	if failErr := o.runner.FailExecution(ctx, exec.ID, failClass, "step_error", err.Error()); failErr != nil {
		execLog.Warn("orchestrator.fail_execution_error", "error", failErr.Error())
	}
}

type action int

const (
	actionNone action = iota
	actionSubmitPlaceholder
	actionAcquireLease
	actionPrecheck
	actionAcquireLeaseAndPrecheck
	actionQuiesce
	actionVerifySourceQuiesce
	actionReconfigure
	actionReboot
	actionSSHPoll
	actionAttach
	actionVerify
	actionComplete
)

func (o *Orchestrator) determineAction(exec *domain.Execution) action {
	switch exec.Direction {
	case domain.DirectionSlurmToOpenStack:
		return o.determineS2O(exec)
	case domain.DirectionOpenStackToSlurm:
		return o.determineO2S(exec)
	}
	return actionNone
}

func (o *Orchestrator) determineS2O(exec *domain.Execution) action {
	switch exec.CurrentState {
	case domain.StateRequested:
		return actionSubmitPlaceholder
	case domain.StateAwaitingSourceAllocation:
		return actionNone // waiting for MQ event
	case domain.StateNodeIdentified:
		return actionAcquireLeaseAndPrecheck
	case domain.StateLocked:
		return actionPrecheck
	case domain.StatePrecheckPassed:
		return actionQuiesce
	case domain.StateSourceQuiescing:
		return actionNone // waiting for MQ event
	case domain.StateSourceDetached:
		return actionReconfigure
	case domain.StateHostReconfiguring:
		return actionReboot
	case domain.StateRebooting:
		return actionSSHPoll
	case domain.StateHostReachable:
		return actionAttach
	case domain.StateTargetAttaching:
		return actionVerify
	case domain.StateVerifying:
		return actionComplete
	}
	return actionNone
}

func (o *Orchestrator) determineO2S(exec *domain.Execution) action {
	switch exec.CurrentState {
	case domain.StateRequested:
		return actionAcquireLease
	case domain.StateLocked:
		return actionPrecheck
	case domain.StatePrecheckPassed:
		return actionQuiesce
	case domain.StateSourceQuiescing:
		return actionVerifySourceQuiesce
	case domain.StateSourceDetached:
		return actionReconfigure
	case domain.StateHostReconfiguring:
		return actionReboot
	case domain.StateRebooting:
		return actionSSHPoll
	case domain.StateHostReachable:
		return actionAttach
	case domain.StateTargetAttaching:
		return actionVerify
	case domain.StateVerifying:
		return actionComplete
	}
	return actionNone
}

func (o *Orchestrator) executeAction(ctx context.Context, exec *domain.Execution, a action) error {
	switch a {
	case actionSubmitPlaceholder:
		return o.doSubmitPlaceholder(ctx, exec)
	case actionAcquireLease:
		return o.doAcquireLease(ctx, exec)
	case actionAcquireLeaseAndPrecheck:
		if err := o.doAcquireLease(ctx, exec); err != nil {
			return err
		}
		fresh, err := o.store.GetExecution(ctx, exec.ID)
		if err != nil {
			return err
		}
		return o.doPrecheck(ctx, fresh)
	case actionPrecheck:
		return o.doPrecheck(ctx, exec)
	case actionQuiesce:
		return o.doQuiesce(ctx, exec)
	case actionVerifySourceQuiesce:
		return o.doVerifySourceQuiesce(ctx, exec)
	case actionReconfigure:
		return o.doReconfigure(ctx, exec)
	case actionReboot:
		return o.doReboot(ctx, exec)
	case actionSSHPoll:
		return o.doSSHPoll(ctx, exec)
	case actionAttach:
		return o.doAttach(ctx, exec)
	case actionVerify:
		return o.doVerify(ctx, exec)
	case actionComplete:
		return o.doComplete(ctx, exec)
	}
	return nil
}

func (o *Orchestrator) doSubmitPlaceholder(ctx context.Context, exec *domain.Execution) error {
	if o.slurm == nil {
		return errors.New("slurm client not configured")
	}
	result, err := o.slurm.SubmitPlaceholderJob(ctx, slurm.PlaceholderJobRequest{
		ExecutionID: exec.ID,
		Constraint:  exec.RequestedSlurmConstraint,
		Partition:   exec.RequestedSlurmPartition,
	})
	if err != nil {
		return err
	}

	exec.PlaceholderJobID = result.JobID
	if err := o.store.UpdateExecution(ctx, exec); err != nil {
		return err
	}

	trace.ForExecution(o.logger, exec).Info(trace.EventWaitEntered,
		"action", "submit_placeholder",
		"wait_for", "allocation_event",
		"job_id", result.JobID,
	)
	return o.runner.Transition(ctx, exec.ID, domain.StateAwaitingSourceAllocation)
}

func (o *Orchestrator) doAcquireLease(ctx context.Context, exec *domain.Execution) error {
	lease := &domain.NodeLease{
		NodeName:     exec.NodeName,
		ExecutionID:  exec.ID,
		Holder:       "orchestrator",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		StateVersion: exec.StateVersion,
	}
	if err := o.store.AcquireLease(ctx, lease); err != nil {
		return err
	}

	now := time.Now()
	exec.LockAcquiredAt = &now
	if err := o.store.UpdateExecution(ctx, exec); err != nil {
		return err
	}

	trace.ForExecution(o.logger, exec).Info(trace.EventActionSucceeded,
		"action", "acquire_lease",
		"node_name", exec.NodeName,
	)
	return o.runner.Transition(ctx, exec.ID, domain.StateLocked)
}

func (o *Orchestrator) doPrecheck(ctx context.Context, exec *domain.Execution) error {
	if o.openstack == nil {
		return errors.New("openstack client not configured")
	}
	_, err := o.openstack.GetComputeService(ctx, exec.NodeName)
	if err != nil {
		return err
	}

	return o.runner.Transition(ctx, exec.ID, domain.StatePrecheckPassed)
}

func (o *Orchestrator) doQuiesce(ctx context.Context, exec *domain.Execution) error {
	if exec.Direction == domain.DirectionSlurmToOpenStack {
		if o.slurm == nil {
			return errors.New("slurm client not configured")
		}
		if err := o.slurm.DrainNode(ctx, exec.NodeName, "gpu-switch:"+exec.ID); err != nil {
			return err
		}
		trace.ForExecution(o.logger, exec).Info(trace.EventWaitEntered,
			"action", "quiesce",
			"wait_for", "drained_event",
		)
	} else {
		if o.openstack == nil {
			return errors.New("openstack client not configured")
		}
		if err := o.openstack.DisableComputeService(ctx, exec.NodeName, "gpu-switch:"+exec.ID); err != nil {
			return err
		}
		trace.ForExecution(o.logger, exec).Info(trace.EventWaitEntered,
			"action", "quiesce",
			"wait_for", "openstack_source_quiesce",
		)
	}

	return o.runner.Transition(ctx, exec.ID, domain.StateSourceQuiescing)
}

func (o *Orchestrator) doVerifySourceQuiesce(ctx context.Context, exec *domain.Execution) error {
	if o.openstack == nil {
		return errors.New("openstack client not configured")
	}

	computeService, err := o.openstack.GetComputeService(ctx, exec.NodeName)
	if err != nil {
		return err
	}

	instances, err := o.openstack.ListInstances(ctx, exec.NodeName)
	if err != nil {
		return err
	}

	activeMigrations, err := o.openstack.ListActiveMigrations(ctx, exec.NodeName)
	if err != nil {
		return err
	}

	if computeService.Enabled || len(instances) > 0 || len(activeMigrations) > 0 {
		trace.ForExecution(o.logger, exec).Info(trace.EventWaitProgress,
			"action", "verify_source_quiesce",
			"wait_for", "openstack_source_quiesce",
			"compute_service_enabled", computeService.Enabled,
			"resident_instances", len(instances),
			"active_migrations", len(activeMigrations),
		)
		return nil
	}

	trace.ForExecution(o.logger, exec).Info(trace.EventWaitSatisfied,
		"action", "verify_source_quiesce",
		"wait_for", "openstack_source_quiesce",
	)
	return o.runner.Transition(ctx, exec.ID, domain.StateSourceDetached)
}

func (o *Orchestrator) doReconfigure(ctx context.Context, exec *domain.Execution) error {
	return o.runner.Transition(ctx, exec.ID, domain.StateHostReconfiguring)
}

func (o *Orchestrator) doReboot(ctx context.Context, exec *domain.Execution) error {
	if o.sshRunner == nil {
		return errors.New("ssh runner not configured")
	}
	_, err := o.sshRunner.Execute(ctx, remote.CommandRequest{
		Host:        exec.NodeName,
		Command:     "reboot",
		ExecutionID: exec.ID,
		StepName:    "reboot",
		Timeout:     30 * time.Second,
	})
	// reboot command may return error as connection drops
	_ = err

	trace.ForExecution(o.logger, exec).Info(trace.EventWaitEntered,
		"action", "reboot",
		"wait_for", "ssh_reachability",
	)
	return o.runner.Transition(ctx, exec.ID, domain.StateRebooting)
}

func (o *Orchestrator) doSSHPoll(ctx context.Context, exec *domain.Execution) error {
	execLog := trace.ForExecution(o.logger, exec)
	err := PollSSHReachable(ctx, o.sshRunner, exec.NodeName, ReachabilityConfig{
		Interval: o.cfg.SSHPollInterval,
		Timeout:  o.cfg.SSHPollTimeout,
	}, o.logger)
	if errors.Is(err, ErrSSHPollTimeout) {
		execLog.Warn(trace.EventWaitTimeout, "action", "ssh_poll")
		return o.runner.FailExecution(ctx, exec.ID, domain.FailureUnknownAfterReboot, "ssh_poll_timeout", "host did not become reachable within timeout")
	}
	if err != nil {
		return err
	}

	return o.runner.Transition(ctx, exec.ID, domain.StateHostReachable)
}

func (o *Orchestrator) doAttach(ctx context.Context, exec *domain.Execution) error {
	if exec.Direction == domain.DirectionSlurmToOpenStack {
		if o.openstack == nil {
			return errors.New("openstack client not configured")
		}
		if err := o.openstack.EnableComputeService(ctx, exec.NodeName); err != nil {
			return err
		}
	} else {
		if o.slurm == nil {
			return errors.New("slurm client not configured")
		}
		if err := o.slurm.ResumeNode(ctx, exec.NodeName); err != nil {
			return err
		}
	}

	return o.runner.Transition(ctx, exec.ID, domain.StateTargetAttaching)
}

func (o *Orchestrator) doVerify(ctx context.Context, exec *domain.Execution) error {
	return o.runner.Transition(ctx, exec.ID, domain.StateVerifying)
}

func (o *Orchestrator) doComplete(ctx context.Context, exec *domain.Execution) error {
	if err := o.store.ReleaseLease(ctx, exec.NodeName, exec.ID); err != nil && !errors.Is(err, store.ErrLeaseNotHeld) {
		return err
	}

	if err := o.runner.Transition(ctx, exec.ID, domain.StateCompleted); err != nil {
		return err
	}

	trace.ForExecution(o.logger, exec).Info(trace.EventExecutionCompleted, "action", "complete")
	return nil
}

func classifyFailure(state domain.SwitchState) domain.FailureClass {
	switch state {
	case domain.StateRebooting:
		return domain.FailureUnknownAfterReboot
	case domain.StateRequested,
		domain.StateAwaitingSourceAllocation,
		domain.StateNodeIdentified,
		domain.StateLocked,
		domain.StatePrecheckPassed,
		domain.StateSourceQuiescing:
		return domain.FailurePrecheckBlocked
	default:
		return domain.FailureMutationPartial
	}
}
