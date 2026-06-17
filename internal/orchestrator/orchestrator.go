package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/openstack"
	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

type worker struct {
	dirty bool
}

type Config struct {
	TickInterval           time.Duration
	SSHPollInterval        time.Duration
	SSHPollTimeout         time.Duration
	PlaceholderSIFPath     string
	PlaceholderSIFFile     string
}

type Orchestrator struct {
	store     store.Store
	runner    *engine.Runner
	steps     *engine.StepTracker
	sshRunner remote.Runner
	slurm     slurm.Client
	openstack openstack.Client
	cfg       Config
	logger    *slog.Logger
	mu        sync.Mutex
	workers   map[string]*worker
	runCtx    context.Context
}

const slurmdCommandTimeout = 30 * time.Second

func New(s store.Store, runner *engine.Runner, sshRunner remote.Runner, slurmClient slurm.Client, osClient openstack.Client, cfg Config, logger *slog.Logger) *Orchestrator {
	l := trace.OrDefault(logger)
	return &Orchestrator{
		store:     s,
		runner:    runner,
		steps:     engine.NewStepTracker(s, l),
		sshRunner: sshRunner,
		slurm:     slurmClient,
		openstack: osClient,
		cfg:       cfg,
		logger:    l,
		workers:   make(map[string]*worker),
	}
}

func (o *Orchestrator) Run(ctx context.Context) {
	o.mu.Lock()
	o.runCtx = ctx
	o.mu.Unlock()
	o.recoverActiveExecutions(ctx)
	<-ctx.Done()
}

func (o *Orchestrator) AdmitExecution(ctx context.Context, executionID string) {
	if executionID == "" {
		return
	}
	o.mu.Lock()
	runCtx := o.runCtx

	if w, exists := o.workers[executionID]; exists {
		w.dirty = true
		o.mu.Unlock()
		return
	}
	o.mu.Unlock()

	// Prefer the long-lived Run context so the worker is not tied to a
	// short-lived caller context (e.g. an HTTP request that returns 202).
	workerCtx := ctx
	if runCtx != nil {
		if runCtx.Err() != nil {
			return
		}
		workerCtx = runCtx
	} else if ctx.Err() != nil {
		return
	}
	if !o.startWorker(executionID) {
		return
	}
	go o.runExecution(workerCtx, executionID)
}

func (o *Orchestrator) recoverActiveExecutions(ctx context.Context) {
	executions, err := o.store.ListActiveExecutions(ctx)
	if err != nil {
		o.logger.Error("orchestrator.list_active_executions_failed", "error", err)
		return
	}

	for _, exec := range executions {
		if ctx.Err() != nil {
			return
		}
		if o.shouldRecover(exec) {
			o.AdmitExecution(ctx, exec.ID)
		}
	}
}

func (o *Orchestrator) startWorker(executionID string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, exists := o.workers[executionID]; exists {
		return false
	}
	o.workers[executionID] = &worker{}
	return true
}

func (o *Orchestrator) finishWorker(executionID string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.workers, executionID)
}

func (o *Orchestrator) runExecution(ctx context.Context, executionID string) {
	defer o.finishWorker(executionID)

	for {
		if ctx.Err() != nil {
			return
		}

		o.mu.Lock()
		w, exists := o.workers[executionID]
		if !exists {
			o.mu.Unlock()
			return
		}
		w.dirty = false
		o.mu.Unlock()

		exec, err := o.store.GetExecution(ctx, executionID)
		if errors.Is(err, store.ErrNotFound) {
			return
		}
		if err != nil {
			o.logger.Warn("orchestrator.get_execution_failed", "execution_id", executionID, "error", err.Error())
			return
		}
		if exec.CurrentState.IsTerminal() || exec.OverallStatus != domain.OverallStatusActive {
			return
		}

		action := o.determineAction(exec)
		if action == actionNone {
			o.mu.Lock()
			if w.dirty {
				o.mu.Unlock()
				continue
			}
			o.mu.Unlock()
			return
		}

		previousState := exec.CurrentState
		previousVersion := exec.StateVersion
		o.processExecution(ctx, exec)

		fresh, err := o.store.GetExecution(ctx, executionID)
		if errors.Is(err, store.ErrNotFound) {
			return
		}
		if err != nil {
			o.logger.Warn("orchestrator.get_execution_failed", "execution_id", executionID, "error", err.Error())
			return
		}
		if fresh.CurrentState.IsTerminal() || fresh.OverallStatus != domain.OverallStatusActive {
			return
		}

		delay, cont := o.nextStep(fresh, previousState, previousVersion)
		if !cont {
			o.mu.Lock()
			if w.dirty {
				o.mu.Unlock()
				continue
			}
			o.mu.Unlock()
			return
		}
		if delay > 0 {
			if err := waitForNextPoll(ctx, delay); err != nil {
				return
			}
		}
	}
}

func (o *Orchestrator) nextStep(exec *domain.Execution, previousState domain.SwitchState, previousVersion int64) (time.Duration, bool) {
	if exec.CurrentState == previousState && exec.StateVersion == previousVersion {
		if exec.Direction == domain.DirectionOpenStackToSlurm && exec.CurrentState == domain.StateSourceQuiescing {
			return o.localPollInterval(), true
		}
		if exec.CurrentState == domain.StateAwaitingSourceAllocation && exec.PlaceholderJobID != "" {
			return o.localPollInterval(), true
		}
		return 0, false
	}

	switch exec.CurrentState {
	case domain.StateAwaitingSourceAllocation:
		if exec.PlaceholderJobID != "" {
			return o.localPollInterval(), true
		}
		return 0, false
	case domain.StateAwaitingTargetNode:
		return 0, false
	case domain.StateSourceQuiescing:
		if exec.Direction == domain.DirectionOpenStackToSlurm {
			return o.localPollInterval(), true
		}
		return 0, false
	default:
		return 0, true
	}
}

func (o *Orchestrator) localPollInterval() time.Duration {
	if o.cfg.TickInterval > 0 {
		return o.cfg.TickInterval
	}
	return 100 * time.Millisecond
}

func waitForNextPoll(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (o *Orchestrator) shouldRecover(exec *domain.Execution) bool {
	switch exec.CurrentState {
	case domain.StateAwaitingSourceAllocation:
		return exec.PlaceholderJobID != ""
	case domain.StateNodeIdentified,
		domain.StateLocked,
		domain.StatePrecheckPassed,
		domain.StateSourceDetached,
		domain.StateHostReconfiguring,
		domain.StateRebooting,
		domain.StateHostReachable,
		domain.StateTargetAttaching,
		domain.StateVerifying,
		domain.StateCancelling:
		return true
	case domain.StateSourceQuiescing:
		return exec.Direction == domain.DirectionOpenStackToSlurm
	default:
		return false
	}
}

func actionName(a action) string {
	switch a {
	case actionSubmitPlaceholder:
		return "submit_placeholder"
	case actionMonitorPlaceholder:
		return "monitor_placeholder"
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
	case actionCancelCleanup:
		return "cancel_cleanup"
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
	errCode := "step_error"
	if exec.CurrentState == domain.StateCancelling {
		errCode = "cancel_cleanup_failed"
	}

	if failClass == domain.FailurePrecheckBlocked && exec.CurrentState != domain.StateCancelling {
		o.bestEffortResourceCleanup(ctx, exec)
	}

	if failErr := o.runner.FailExecution(ctx, exec.ID, failClass, errCode, err.Error()); failErr != nil {
		execLog.Warn("orchestrator.fail_execution_error", "error", failErr.Error())
	}
}

type action int

const (
	actionNone action = iota
	actionSubmitPlaceholder
	actionMonitorPlaceholder
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
	actionCancelCleanup
)

func (o *Orchestrator) determineAction(exec *domain.Execution) action {
	if exec.CurrentState == domain.StateCancelling {
		return actionCancelCleanup
	}
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
		if exec.PlaceholderJobID != "" {
			return actionMonitorPlaceholder
		}
		return actionNone
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
	case domain.StateAwaitingTargetNode:
		return actionNone
	case domain.StateRequested:
		return actionAcquireLease
	case domain.StateNodeIdentified:
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
	case actionMonitorPlaceholder:
		return o.doMonitorPlaceholder(ctx, exec)
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
	case actionCancelCleanup:
		return o.doCancelCleanup(ctx, exec)
	}
	return nil
}

func (o *Orchestrator) doSubmitPlaceholder(ctx context.Context, exec *domain.Execution) error {
	step, err := o.steps.StartStep(ctx, exec.ID, domain.StepSubmitPlaceholder, exec.NodeName)
	if err != nil {
		return err
	}

	if o.slurm == nil {
		return o.failStep(ctx, step, errors.New("slurm client not configured"))
	}
	effectiveSIFFile := exec.PlaceholderSIFFile
	if effectiveSIFFile == "" {
		effectiveSIFFile = o.cfg.PlaceholderSIFFile
	}
	result, err := o.slurm.SubmitPlaceholderJob(ctx, slurm.PlaceholderJobRequest{
		ExecutionID:        exec.ID,
		Constraint:         exec.RequestedSlurmConstraint,
		Partition:          exec.RequestedSlurmPartition,
		Account:            exec.RequestedSlurmAccount,
		WorkloadUser:       exec.SlurmWorkloadUser,
		WorkloadToken:      exec.SlurmWorkloadToken,
		PlaceholderSIFFile: effectiveSIFFile,
	})
	if err != nil {
		return o.failStep(ctx, step, err)
	}

	exec.PlaceholderJobID = result.JobID
	if err := o.store.UpdateExecution(ctx, exec); err != nil {
		return o.failStep(ctx, step, err)
	}

	if err := o.steps.FinishStep(ctx, step, domain.StepStatusSucceeded); err != nil {
		return err
	}

	trace.ForExecution(o.logger, exec).Info(trace.EventWaitEntered,
		"action", "submit_placeholder",
		"wait_for", "allocation_event",
		"job_id", result.JobID,
	)
	if err := o.runner.Transition(ctx, exec.ID, domain.StateAwaitingSourceAllocation); err != nil {
		return err
	}
	_, _ = o.steps.StartStep(ctx, exec.ID, domain.StepWaitForSourceAllocation, exec.NodeName)
	return nil
}

func (o *Orchestrator) doMonitorPlaceholder(ctx context.Context, exec *domain.Execution) error {
	if o.slurm == nil {
		return nil
	}

	id := slurm.WorkloadIdentity{
		User:  exec.SlurmWorkloadUser,
		Token: exec.SlurmWorkloadToken,
	}
	jobState, err := o.slurm.GetJobState(ctx, exec.PlaceholderJobID, id)
	if err != nil {
		if slurm.IsJobNotFound(err) {
			summary := fmt.Sprintf("placeholder job %s not found while waiting for allocation", exec.PlaceholderJobID)
			return o.failAllocationWait(ctx, exec, summary)
		}
		// transient Slurm error — log and retry on next tick
		trace.ForExecution(o.logger, exec).Warn("monitor_placeholder.job_state_error",
			"job_id", exec.PlaceholderJobID, "error", err.Error())
		return nil
	}

	if !jobState.IsTerminal {
		return nil
	}

	// Re-read the execution to guard against a concurrent MQ allocation event
	// that may have already advanced the state to node_identified. If the
	// allocation event won the race, this poll is a harmless no-op.
	fresh, err := o.store.GetExecution(ctx, exec.ID)
	if err != nil {
		return err
	}
	if fresh.CurrentState != domain.StateAwaitingSourceAllocation {
		return nil
	}

	summary := fmt.Sprintf("placeholder job %s reached terminal state %s before allocation event", exec.PlaceholderJobID, jobState.State)
	return o.failAllocationWait(ctx, fresh, summary)
}

func (o *Orchestrator) failAllocationWait(ctx context.Context, exec *domain.Execution, summary string) error {
	_ = o.steps.CloseRunningStep(ctx, exec.ID, domain.StepStatusFailed,
		engine.WithErrorClass(domain.FailurePrecheckBlocked),
		engine.WithErrorSummary(summary))
	return o.runner.FailExecution(ctx, exec.ID, domain.FailurePrecheckBlocked, "placeholder_job_failed", summary)
}

func (o *Orchestrator) doAcquireLease(ctx context.Context, exec *domain.Execution) error {
	step, err := o.steps.StartStep(ctx, exec.ID, domain.StepAcquireLease, exec.NodeName)
	if err != nil {
		return err
	}

	lease := &domain.NodeLease{
		NodeName:     exec.NodeName,
		ExecutionID:  exec.ID,
		Holder:       "orchestrator",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		StateVersion: exec.StateVersion,
	}
	if err := o.store.AcquireLease(ctx, lease); err != nil {
		return o.failStep(ctx, step, err)
	}

	now := time.Now()
	exec.LockAcquiredAt = &now
	if err := o.store.UpdateExecution(ctx, exec); err != nil {
		return o.failStep(ctx, step, err)
	}

	if err := o.steps.FinishStep(ctx, step, domain.StepStatusSucceeded); err != nil {
		return err
	}

	trace.ForExecution(o.logger, exec).Info(trace.EventActionSucceeded,
		"action", "acquire_lease",
		"node_name", exec.NodeName,
	)
	return o.runner.Transition(ctx, exec.ID, domain.StateLocked)
}

func (o *Orchestrator) doPrecheck(ctx context.Context, exec *domain.Execution) error {
	step, err := o.steps.StartStep(ctx, exec.ID, domain.StepPrecheck, exec.NodeName)
	if err != nil {
		return err
	}

	if o.openstack == nil {
		return o.failPrecheck(ctx, exec, step, "openstack client not configured")
	}

	if exec.Direction == domain.DirectionOpenStackToSlurm {
		result, err := openstack.EvaluateSourceReadiness(ctx, o.openstack, exec.NodeName)
		if err != nil {
			summary := fmt.Sprintf("source readiness check failed: %s", conciseError(err))
			return o.failPrecheck(ctx, exec, step, summary)
		}
		if !result.Ready {
			return o.failPrecheck(ctx, exec, step, result.ErrorSummary())
		}
	} else {
		_, err = o.openstack.GetComputeService(ctx, exec.NodeName)
		if err != nil {
			summary := fmt.Sprintf("compute service on %s: %s", exec.NodeName, conciseError(err))
			return o.failPrecheck(ctx, exec, step, summary)
		}
	}

	if err := o.steps.FinishStep(ctx, step, domain.StepStatusSucceeded); err != nil {
		return err
	}
	return o.runner.Transition(ctx, exec.ID, domain.StatePrecheckPassed)
}

func conciseError(err error) string {
	msg := err.Error()
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		inner := unwrapped.Error()
		if inner != "" {
			return inner
		}
	}
	return msg
}

func (o *Orchestrator) doQuiesce(ctx context.Context, exec *domain.Execution) error {
	step, err := o.steps.StartStep(ctx, exec.ID, domain.StepQuiesceSource, exec.NodeName)
	if err != nil {
		return err
	}

	if exec.Direction == domain.DirectionSlurmToOpenStack {
		if o.slurm == nil {
			return o.failStep(ctx, step, errors.New("slurm client not configured"))
		}
		if err := o.slurm.DrainNode(ctx, exec.NodeName, "gpu-switch:"+exec.ID); err != nil {
			return o.failStep(ctx, step, err)
		}
		trace.ForExecution(o.logger, exec).Info(trace.EventWaitEntered,
			"action", "quiesce",
			"wait_for", "drained_event",
		)
	} else {
		if o.openstack == nil {
			return o.failStep(ctx, step, errors.New("openstack client not configured"))
		}
		if err := o.openstack.DisableComputeService(ctx, exec.NodeName, "gpu-switch:"+exec.ID); err != nil {
			return o.failStep(ctx, step, err)
		}
		trace.ForExecution(o.logger, exec).Info(trace.EventWaitEntered,
			"action", "quiesce",
			"wait_for", "openstack_source_quiesce",
		)
	}

	if err := o.steps.FinishStep(ctx, step, domain.StepStatusSucceeded); err != nil {
		return err
	}
	if err := o.runner.Transition(ctx, exec.ID, domain.StateSourceQuiescing); err != nil {
		return err
	}
	_, _ = o.steps.StartStep(ctx, exec.ID, domain.StepWaitForSourceDrain, exec.NodeName)
	return nil
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

	_ = o.steps.CloseRunningStep(ctx, exec.ID, domain.StepStatusSucceeded)
	trace.ForExecution(o.logger, exec).Info(trace.EventWaitSatisfied,
		"action", "verify_source_quiesce",
		"wait_for", "openstack_source_quiesce",
	)
	return o.runner.Transition(ctx, exec.ID, domain.StateSourceDetached)
}

func (o *Orchestrator) doReconfigure(ctx context.Context, exec *domain.Execution) error {
	step, err := o.steps.StartStep(ctx, exec.ID, domain.StepReconfigureHost, exec.NodeName)
	if err != nil {
		return err
	}

	if exec.Direction == domain.DirectionSlurmToOpenStack {
		if err := o.runSlurmdServiceCommands(ctx, exec, "stop", "disable"); err != nil {
			return o.failStep(ctx, step, err)
		}
	}

	if err := o.steps.FinishStep(ctx, step, domain.StepStatusSucceeded); err != nil {
		return err
	}
	return o.runner.Transition(ctx, exec.ID, domain.StateHostReconfiguring)
}

func (o *Orchestrator) runSlurmdServiceCommands(ctx context.Context, exec *domain.Execution, actions ...string) error {
	for _, action := range actions {
		if err := o.runSlurmdServiceCommand(ctx, exec, action); err != nil {
			return err
		}
	}
	return nil
}

func (o *Orchestrator) runSlurmdServiceCommand(ctx context.Context, exec *domain.Execution, action string) error {
	if o.sshRunner == nil {
		return errors.New("ssh runner not configured")
	}

	result, err := o.sshRunner.Execute(ctx, remote.CommandRequest{
		Host:        exec.NodeName,
		Command:     "systemctl",
		Args:        []string{action, "slurmd"},
		ExecutionID: exec.ID,
		StepName:    "slurmd_" + action,
		Timeout:     slurmdCommandTimeout,
	})
	if err != nil {
		return fmt.Errorf("slurmd %s failed: %w", action, err)
	}
	if result == nil {
		return fmt.Errorf("slurmd %s failed: empty ssh result", action)
	}
	if result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		if message == "" {
			return fmt.Errorf("slurmd %s failed: exit code %d", action, result.ExitCode)
		}
		return fmt.Errorf("slurmd %s failed: exit code %d: %s", action, result.ExitCode, message)
	}

	return nil
}

func (o *Orchestrator) doReboot(ctx context.Context, exec *domain.Execution) error {
	step, err := o.steps.StartStep(ctx, exec.ID, domain.StepReboot, exec.NodeName)
	if err != nil {
		return err
	}

	if o.sshRunner == nil {
		return o.failStep(ctx, step, errors.New("ssh runner not configured"))
	}
	_, err = o.sshRunner.Execute(ctx, remote.CommandRequest{
		Host:        exec.NodeName,
		Command:     "reboot",
		ExecutionID: exec.ID,
		StepName:    "reboot",
		Timeout:     30 * time.Second,
	})
	// reboot command may return error as connection drops
	_ = err

	if err := o.steps.FinishStep(ctx, step, domain.StepStatusSucceeded); err != nil {
		return err
	}
	trace.ForExecution(o.logger, exec).Info(trace.EventWaitEntered,
		"action", "reboot",
		"wait_for", "ssh_reachability",
	)
	return o.runner.Transition(ctx, exec.ID, domain.StateRebooting)
}

func (o *Orchestrator) doSSHPoll(ctx context.Context, exec *domain.Execution) error {
	step, err := o.steps.StartStep(ctx, exec.ID, domain.StepWaitForSSHReachability, exec.NodeName)
	if err != nil {
		return err
	}

	execLog := trace.ForExecution(o.logger, exec)
	pollErr := PollSSHReachable(ctx, o.sshRunner, exec.NodeName, exec.ID, ReachabilityConfig{
		Interval: o.cfg.SSHPollInterval,
		Timeout:  o.cfg.SSHPollTimeout,
	}, o.logger)
	if errors.Is(pollErr, ErrSSHPollTimeout) {
		_ = o.steps.FinishStep(ctx, step, domain.StepStatusFailed, engine.WithErrorClass(domain.FailureUnknownAfterReboot))
		execLog.Warn(trace.EventWaitTimeout, "action", "ssh_poll")
		return o.runner.FailExecution(ctx, exec.ID, domain.FailureUnknownAfterReboot, "ssh_poll_timeout", "host did not become reachable within timeout")
	}
	if pollErr != nil {
		return o.failStep(ctx, step, pollErr)
	}

	if err := o.steps.FinishStep(ctx, step, domain.StepStatusSucceeded); err != nil {
		return err
	}
	return o.runner.Transition(ctx, exec.ID, domain.StateHostReachable)
}

func (o *Orchestrator) doAttach(ctx context.Context, exec *domain.Execution) error {
	step, err := o.steps.StartStep(ctx, exec.ID, domain.StepAttachTarget, exec.NodeName)
	if err != nil {
		return err
	}

	if exec.Direction == domain.DirectionSlurmToOpenStack {
		if o.openstack == nil {
			return o.failStep(ctx, step, errors.New("openstack client not configured"))
		}
		if err := o.openstack.EnableComputeService(ctx, exec.NodeName); err != nil {
			return o.failStep(ctx, step, err)
		}
	} else {
		if err := o.runSlurmdServiceCommands(ctx, exec, "enable", "start"); err != nil {
			return o.failStep(ctx, step, err)
		}
		if o.slurm == nil {
			return o.failStep(ctx, step, errors.New("slurm client not configured"))
		}
		if err := slurm.EnsureNodeReadyForAttach(ctx, o.slurm, exec.NodeName); err != nil {
			return o.failStep(ctx, step, err)
		}
	}

	if err := o.steps.FinishStep(ctx, step, domain.StepStatusSucceeded); err != nil {
		return err
	}
	return o.runner.Transition(ctx, exec.ID, domain.StateTargetAttaching)
}

func (o *Orchestrator) doVerify(ctx context.Context, exec *domain.Execution) error {
	step, err := o.steps.StartStep(ctx, exec.ID, domain.StepVerifyTarget, exec.NodeName)
	if err != nil {
		return err
	}
	if err := o.steps.FinishStep(ctx, step, domain.StepStatusSucceeded); err != nil {
		return err
	}
	return o.runner.Transition(ctx, exec.ID, domain.StateVerifying)
}

func (o *Orchestrator) doComplete(ctx context.Context, exec *domain.Execution) error {
	step, err := o.steps.StartStep(ctx, exec.ID, domain.StepCompleteExecution, exec.NodeName)
	if err != nil {
		return err
	}

	if err := o.store.ReleaseLease(ctx, exec.NodeName, exec.ID); err != nil && !errors.Is(err, store.ErrLeaseNotHeld) {
		return o.failStep(ctx, step, err)
	}

	if err := o.steps.FinishStep(ctx, step, domain.StepStatusSucceeded); err != nil {
		return err
	}

	if err := o.runner.Transition(ctx, exec.ID, domain.StateCompleted); err != nil {
		return err
	}

	fresh, err := o.store.GetExecution(ctx, exec.ID)
	if err != nil {
		return fmt.Errorf("re-reading execution: %w", err)
	}

	trace.ForExecution(o.logger, fresh).Info(trace.EventExecutionCompleted, "action", "complete")
	return nil
}

func (o *Orchestrator) doCancelCleanup(ctx context.Context, exec *domain.Execution) error {
	_ = o.steps.CloseRunningStep(ctx, exec.ID, domain.StepStatusSkipped)

	step, err := o.steps.StartStep(ctx, exec.ID, domain.StepCancelCleanup, exec.NodeName)
	if err != nil {
		return err
	}

	src := exec.CancellationSourceState
	execLog := trace.ForExecution(o.logger, exec)
	execLog.Info("cancel.cleanup_started",
		"source_state", string(src),
		"direction", string(exec.Direction),
	)

	// Phase 1: source-state-driven rollback actions.
	if err := o.cancelSourceRollback(ctx, exec, step); err != nil {
		return err
	}

	// Phase 2: resource-driven teardown — always clean up owned resources
	// regardless of the source state that accepted the cancellation.
	if err := o.cancelResourceTeardown(ctx, exec, step); err != nil {
		return err
	}

	if err := o.steps.FinishStep(ctx, step, domain.StepStatusSucceeded); err != nil {
		return err
	}
	execLog.Info("cancel.cleanup_succeeded", "source_state", string(src))

	if err := o.runner.Transition(ctx, exec.ID, domain.StateCancelled); err != nil {
		return fmt.Errorf("transitioning to cancelled: %w", err)
	}

	fresh, err := o.store.GetExecution(ctx, exec.ID)
	if err != nil {
		return fmt.Errorf("re-reading execution after cancel: %w", err)
	}
	fresh.FinalErrorCode = "cancelled_by_user"
	fresh.FinalErrorSummary = fmt.Sprintf("execution cancelled while in %s", string(src))
	if err := o.store.UpdateExecution(ctx, fresh); err != nil {
		return fmt.Errorf("recording cancellation outcome: %w", err)
	}

	trace.ForExecution(o.logger, fresh).Info("cancel.execution_cancelled",
		"source_state", string(src),
		"final_error_code", "cancelled_by_user",
	)
	return nil
}

// cancelSourceRollback performs source-state-specific rollback actions (e.g.,
// resuming a Slurm node or re-enabling an OpenStack compute service).
func (o *Orchestrator) cancelSourceRollback(ctx context.Context, exec *domain.Execution, step *domain.StepRecord) error {
	src := exec.CancellationSourceState

	switch src {
	case domain.StateAwaitingTargetNode, domain.StateAwaitingSourceAllocation:
		// No source rollback needed for these states.

	case domain.StateSourceQuiescing:
		switch exec.Direction {
		case domain.DirectionSlurmToOpenStack:
			if o.slurm == nil {
				return o.failStep(ctx, step, errors.New("slurm client not configured"))
			}
			if err := o.slurm.ResumeNode(ctx, exec.NodeName); err != nil {
				return o.failStep(ctx, step, fmt.Errorf("resuming slurm node: %w", err))
			}

		case domain.DirectionOpenStackToSlurm:
			if o.openstack == nil {
				return o.failStep(ctx, step, errors.New("openstack client not configured"))
			}
			if err := o.openstack.EnableComputeService(ctx, exec.NodeName); err != nil {
				return o.failStep(ctx, step, fmt.Errorf("re-enabling openstack compute service: %w", err))
			}
		}

	default:
		return o.failStep(ctx, step, fmt.Errorf("unknown cancellation source state: %s", src))
	}

	return nil
}

// cancelResourceTeardown tears down any execution-owned resources (placeholder
// job and lease) based on what is currently attached to the execution, not the
// source state. Already-absent resources are treated as successfully cleaned.
func (o *Orchestrator) cancelResourceTeardown(ctx context.Context, exec *domain.Execution, step *domain.StepRecord) error {
	if exec.PlaceholderJobID != "" {
		if o.slurm == nil {
			return o.failStep(ctx, step, errors.New("slurm client not configured"))
		}
		if err := o.cancelJobWithExecIdentity(ctx, exec); err != nil {
			if !slurm.IsJobNotFound(err) {
				return o.failStep(ctx, step, fmt.Errorf("cancelling placeholder job %s: %w", exec.PlaceholderJobID, err))
			}
		}
	}

	if err := o.store.ReleaseLease(ctx, exec.NodeName, exec.ID); err != nil && !errors.Is(err, store.ErrLeaseNotHeld) {
		return o.failStep(ctx, step, fmt.Errorf("releasing lease: %w", err))
	}

	return nil
}

// bestEffortResourceCleanup releases placeholder job and lease on a best-effort
// basis when an execution fails before any destructive mutation. Errors are
// logged but do not prevent the execution from being terminalized.
func (o *Orchestrator) bestEffortResourceCleanup(ctx context.Context, exec *domain.Execution) {
	execLog := trace.ForExecution(o.logger, exec)

	if exec.PlaceholderJobID != "" && o.slurm != nil {
		if err := o.cancelJobWithExecIdentity(ctx, exec); err != nil && !slurm.IsJobNotFound(err) {
			execLog.Warn("cleanup.cancel_placeholder_job_failed", "job_id", exec.PlaceholderJobID, "error", err.Error())
		}
	}

	if err := o.store.ReleaseLease(ctx, exec.NodeName, exec.ID); err != nil && !errors.Is(err, store.ErrLeaseNotHeld) {
		execLog.Warn("cleanup.release_lease_failed", "node_name", exec.NodeName, "error", err.Error())
	}
}

func (o *Orchestrator) cancelJobWithExecIdentity(ctx context.Context, exec *domain.Execution) error {
	if exec.SlurmWorkloadUser != "" && exec.SlurmWorkloadToken != "" {
		return o.slurm.CancelJobWithIdentity(ctx, exec.PlaceholderJobID, slurm.WorkloadIdentity{
			User:  exec.SlurmWorkloadUser,
			Token: exec.SlurmWorkloadToken,
		})
	}
	return o.slurm.CancelJob(ctx, exec.PlaceholderJobID)
}

func (o *Orchestrator) failStep(ctx context.Context, step *domain.StepRecord, err error) error {
	_ = o.steps.FinishStep(ctx, step, domain.StepStatusFailed)
	return err
}

func (o *Orchestrator) failPrecheck(ctx context.Context, exec *domain.Execution, step *domain.StepRecord, summary string) error {
	_ = o.steps.FinishStep(ctx, step, domain.StepStatusFailed,
		engine.WithErrorClass(domain.FailurePrecheckBlocked),
		engine.WithErrorSummary(summary))
	o.bestEffortResourceCleanup(ctx, exec)
	return o.runner.FailExecution(ctx, exec.ID, domain.FailurePrecheckBlocked, "precheck_blocked", summary)
}

func classifyFailure(state domain.SwitchState) domain.FailureClass {
	switch state {
	case domain.StateRebooting:
		return domain.FailureUnknownAfterReboot
	case domain.StateRequested,
		domain.StateAwaitingSourceAllocation,
		domain.StateAwaitingTargetNode,
		domain.StateNodeIdentified,
		domain.StateLocked,
		domain.StatePrecheckPassed,
		domain.StateSourceQuiescing:
		return domain.FailurePrecheckBlocked
	case domain.StateCancelling:
		return domain.FailurePrecheckBlocked
	default:
		return domain.FailureMutationPartial
	}
}
