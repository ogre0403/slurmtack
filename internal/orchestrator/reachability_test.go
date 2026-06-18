package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

type recordingSSHRunner struct {
	requests []remote.CommandRequest
	result   *remote.CommandResult
	err      error
}

func (r *recordingSSHRunner) Execute(_ context.Context, req remote.CommandRequest) (*remote.CommandResult, error) {
	r.requests = append(r.requests, req)
	if r.result != nil {
		return r.result, r.err
	}
	return &remote.CommandResult{ExitCode: 0}, r.err
}

type scriptedSSHResponse struct {
	result *remote.CommandResult
	err    error
}

type scriptedSSHRunner struct {
	t              *testing.T
	requests       []remote.CommandRequest
	responses      []scriptedSSHResponse
	defaultResp    *scriptedSSHResponse
	onBeforeReturn func(call int, req remote.CommandRequest)
}

func (r *scriptedSSHRunner) Execute(_ context.Context, req remote.CommandRequest) (*remote.CommandResult, error) {
	r.t.Helper()
	r.requests = append(r.requests, req)
	call := len(r.requests)
	if r.onBeforeReturn != nil {
		r.onBeforeReturn(call, req)
	}
	if len(r.responses) == 0 {
		if r.defaultResp != nil {
			if r.defaultResp.result != nil {
				return r.defaultResp.result, r.defaultResp.err
			}
			return &remote.CommandResult{ExitCode: 0}, r.defaultResp.err
		}
		return &remote.CommandResult{ExitCode: 0}, nil
	}
	resp := r.responses[0]
	r.responses = r.responses[1:]
	if resp.result != nil {
		return resp.result, resp.err
	}
	return &remote.CommandResult{ExitCode: 0}, resp.err
}

func newExecutionForState(state domain.SwitchState) *domain.Execution {
	return &domain.Execution{
		ID:            "ssh-exec-1",
		NodeName:      "gpu-node-01",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  state,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
		StateVersion:  7,
		OverallStatus: domain.OverallStatusActive,
	}
}

func newSSHOrchestrator(t *testing.T, sshRunner remote.Runner, logger *slog.Logger) (*Orchestrator, store.Store, *domain.Execution) {
	t.Helper()

	s := store.NewMemoryStore()
	exec := newExecutionForState(domain.StateHostReconfiguring)
	ctx := context.Background()
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	runner := engine.NewRunner(s, logger)
	orch := New(s, runner, sshRunner, nil, nil, Config{
		SSHPollInterval: time.Millisecond,
		SSHPollTimeout:  50 * time.Millisecond,
	}, logger)

	return orch, s, exec
}

func TestDoRebootDispatchesExactCommand(t *testing.T) {
	logger, _ := newCaptureLogger()
	sshRunner := &recordingSSHRunner{}
	orch, s, exec := newSSHOrchestrator(t, sshRunner, logger)

	if err := orch.doReboot(context.Background(), exec); err != nil {
		t.Fatalf("doReboot() error = %v", err)
	}
	if len(sshRunner.requests) != 1 {
		t.Fatalf("sshRunner requests = %d, want 1", len(sshRunner.requests))
	}
	req := sshRunner.requests[0]
	if req.Host != exec.NodeName {
		t.Fatalf("reboot host = %q, want %q", req.Host, exec.NodeName)
	}
	if req.Command != "reboot" {
		t.Fatalf("reboot command = %q, want %q", req.Command, "reboot")
	}
	if len(req.Args) != 0 {
		t.Fatalf("reboot args = %#v, want none", req.Args)
	}
	if req.ExecutionID != exec.ID {
		t.Fatalf("reboot execution_id = %q, want %q", req.ExecutionID, exec.ID)
	}
	if req.StepName != "reboot" {
		t.Fatalf("reboot step_name = %q, want %q", req.StepName, "reboot")
	}

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateRebooting {
		t.Fatalf("execution state = %s, want %s", updated.CurrentState, domain.StateRebooting)
	}
}

func TestDoReconfigureStopsAndDisablesSlurmdBeforeTransition(t *testing.T) {
	logger, _ := newCaptureLogger()
	sshRunner := &recordingSSHRunner{}
	orch, s, exec := newSSHOrchestrator(t, sshRunner, logger)
	exec.Direction = domain.DirectionSlurmToOpenStack
	exec.CurrentState = domain.StateSourceDetached
	exec.DesiredOwner = domain.OwnerOpenStack
	exec.PreviousOwner = domain.OwnerSlurm
	if err := s.UpdateExecution(context.Background(), exec); err != nil {
		t.Fatalf("update execution: %v", err)
	}

	if err := orch.doReconfigure(context.Background(), exec); err != nil {
		t.Fatalf("doReconfigure() error = %v", err)
	}
	if len(sshRunner.requests) != 2 {
		t.Fatalf("sshRunner requests = %d, want 2", len(sshRunner.requests))
	}
	for i, want := range []struct {
		action string
		step   string
	}{
		{action: "stop", step: "slurmd_stop"},
		{action: "disable", step: "slurmd_disable"},
	} {
		req := sshRunner.requests[i]
		if req.Host != exec.NodeName {
			t.Fatalf("request %d host = %q, want %q", i+1, req.Host, exec.NodeName)
		}
		if req.Command != "systemctl" {
			t.Fatalf("request %d command = %q, want %q", i+1, req.Command, "systemctl")
		}
		if len(req.Args) != 2 || req.Args[0] != want.action || req.Args[1] != "slurmd" {
			t.Fatalf("request %d args = %#v, want [%q %q]", i+1, req.Args, want.action, "slurmd")
		}
		if req.ExecutionID != exec.ID {
			t.Fatalf("request %d execution_id = %q, want %q", i+1, req.ExecutionID, exec.ID)
		}
		if req.StepName != want.step {
			t.Fatalf("request %d step_name = %q, want %q", i+1, req.StepName, want.step)
		}
		if req.Timeout != slurmdCommandTimeout {
			t.Fatalf("request %d timeout = %s, want %s", i+1, req.Timeout, slurmdCommandTimeout)
		}
	}

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateHostReconfiguring {
		t.Fatalf("execution state = %s, want %s", updated.CurrentState, domain.StateHostReconfiguring)
	}
}

func TestDoReconfigureBlocksWhenSlurmdStopFails(t *testing.T) {
	logger, _ := newCaptureLogger()
	sshRunner := &scriptedSSHRunner{
		t: t,
		responses: []scriptedSSHResponse{{
			result: &remote.CommandResult{ExitCode: 1, Stderr: "systemd offline"},
		}},
	}
	orch, s, exec := newSSHOrchestrator(t, sshRunner, logger)
	exec.Direction = domain.DirectionSlurmToOpenStack
	exec.CurrentState = domain.StateSourceDetached
	exec.DesiredOwner = domain.OwnerOpenStack
	exec.PreviousOwner = domain.OwnerSlurm
	if err := s.UpdateExecution(context.Background(), exec); err != nil {
		t.Fatalf("update execution: %v", err)
	}

	err := orch.doReconfigure(context.Background(), exec)
	if err == nil {
		t.Fatal("doReconfigure() error = nil, want slurmd stop failure")
	}
	if err.Error() != "slurmd stop failed: exit code 1: systemd offline" {
		t.Fatalf("doReconfigure() error = %q, want %q", err.Error(), "slurmd stop failed: exit code 1: systemd offline")
	}
	if len(sshRunner.requests) != 1 {
		t.Fatalf("sshRunner requests = %d, want 1", len(sshRunner.requests))
	}

	updated, getErr := s.GetExecution(context.Background(), exec.ID)
	if getErr != nil {
		t.Fatalf("get execution: %v", getErr)
	}
	if updated.CurrentState != domain.StateSourceDetached {
		t.Fatalf("execution state = %s, want %s", updated.CurrentState, domain.StateSourceDetached)
	}
}

func TestDoSSHPollThreadsExecutionMetadataIntoProbe(t *testing.T) {
	logger, _ := newCaptureLogger()
	sshRunner := &scriptedSSHRunner{
		t: t,
		responses: []scriptedSSHResponse{{
			err: errors.New("connection refused"),
		}, {
			result: &remote.CommandResult{ExitCode: 0},
		}},
	}
	orch, s, exec := newSSHOrchestrator(t, sshRunner, logger)
	exec.CurrentState = domain.StateRebooting
	if err := s.UpdateExecution(context.Background(), exec); err != nil {
		t.Fatalf("update execution: %v", err)
	}

	if err := orch.doSSHPoll(context.Background(), exec); err != nil {
		t.Fatalf("doSSHPoll() error = %v", err)
	}
	if len(sshRunner.requests) != 2 {
		t.Fatalf("sshRunner requests = %d, want 2", len(sshRunner.requests))
	}
	req := sshRunner.requests[0]
	if req.Host != exec.NodeName {
		t.Fatalf("probe host = %q, want %q", req.Host, exec.NodeName)
	}
	if req.Command != probeCommand {
		t.Fatalf("probe command = %q, want %q", req.Command, probeCommand)
	}
	if len(req.Args) != len(probeArgs) {
		t.Fatalf("probe args = %#v, want %#v", req.Args, probeArgs)
	}
	for i, want := range probeArgs {
		if req.Args[i] != want {
			t.Fatalf("probe arg %d = %q, want %q", i, req.Args[i], want)
		}
	}
	if req.ExecutionID != exec.ID {
		t.Fatalf("probe execution_id = %q, want %q", req.ExecutionID, exec.ID)
	}
	if req.StepName != sshProbeStepName {
		t.Fatalf("probe step_name = %q, want %q", req.StepName, sshProbeStepName)
	}
	for i, probeReq := range sshRunner.requests[1:] {
		if probeReq.ExecutionID != exec.ID {
			t.Fatalf("probe %d execution_id = %q, want %q", i+2, probeReq.ExecutionID, exec.ID)
		}
		if probeReq.StepName != sshProbeStepName {
			t.Fatalf("probe %d step_name = %q, want %q", i+2, probeReq.StepName, sshProbeStepName)
		}
	}

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateHostReachable {
		t.Fatalf("execution state = %s, want %s", updated.CurrentState, domain.StateHostReachable)
	}
}

func TestRunRecoversRebootingExecution(t *testing.T) {
	logger, captured := newCaptureLogger()
	sshRunner := &scriptedSSHRunner{t: t}
	orch, s, exec := newSSHOrchestrator(t, sshRunner, logger)
	exec.CurrentState = domain.StateRebooting
	if err := s.UpdateExecution(context.Background(), exec); err != nil {
		t.Fatalf("update execution: %v", err)
	}

	runCtx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	orch.Run(runCtx)

	selected := captured.find(trace.EventActionSelected)
	if selected == nil {
		t.Fatal("expected action.selected log")
	}
	if selected.Attrs["action"] != "ssh_poll" {
		t.Fatalf("action.selected action = %q, want %q", selected.Attrs["action"], "ssh_poll")
	}
}

func TestDoSSHPollIgnoresEarlySuccessUntilHostReturns(t *testing.T) {
	logger, captured := newCaptureLogger()
	sshRunner := &scriptedSSHRunner{
		t: t,
		responses: []scriptedSSHResponse{{
			result: &remote.CommandResult{ExitCode: 0},
		}, {
			result: &remote.CommandResult{ExitCode: 0},
		}, {
			err: errors.New("connection refused"),
		}, {
			result: &remote.CommandResult{ExitCode: 0},
		}},
	}
	orch, s, exec := newSSHOrchestrator(t, sshRunner, logger)
	exec.CurrentState = domain.StateRebooting
	if err := s.UpdateExecution(context.Background(), exec); err != nil {
		t.Fatalf("update execution: %v", err)
	}
	sshRunner.onBeforeReturn = func(call int, _ remote.CommandRequest) {
		updated, err := s.GetExecution(context.Background(), exec.ID)
		if err != nil {
			t.Fatalf("get execution during probe %d: %v", call, err)
		}
		if updated.CurrentState != domain.StateRebooting {
			t.Fatalf("execution state during probe %d = %s, want %s", call, updated.CurrentState, domain.StateRebooting)
		}
	}

	if err := orch.doSSHPoll(context.Background(), exec); err != nil {
		t.Fatalf("doSSHPoll() error = %v", err)
	}
	if len(sshRunner.requests) != 4 {
		t.Fatalf("sshRunner requests = %d, want 4", len(sshRunner.requests))
	}

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateHostReachable {
		t.Fatalf("execution state = %s, want %s", updated.CurrentState, domain.StateHostReachable)
	}

	progressLogs := captured.findAll("wait.progress")
	if len(progressLogs) < 3 {
		t.Fatalf("wait.progress logs = %d, want at least 3", len(progressLogs))
	}
	if progressLogs[0].Attrs["probe_result"] != "ignored_early_success" {
		t.Fatalf("first wait.progress probe_result = %q, want %q", progressLogs[0].Attrs["probe_result"], "ignored_early_success")
	}
	if progressLogs[1].Attrs["probe_result"] != "ignored_early_success" {
		t.Fatalf("second wait.progress probe_result = %q, want %q", progressLogs[1].Attrs["probe_result"], "ignored_early_success")
	}
	if progressLogs[2].Attrs["probe_result"] != "reboot_progress_observed" {
		t.Fatalf("third wait.progress probe_result = %q, want %q", progressLogs[2].Attrs["probe_result"], "reboot_progress_observed")
	}

	satisfied := captured.find("wait.satisfied")
	if satisfied == nil {
		t.Fatal("expected wait.satisfied log, got none")
	}
	if satisfied.Attrs["probe_result"] != "post_reboot_recovery" {
		t.Fatalf("wait.satisfied probe_result = %q, want %q", satisfied.Attrs["probe_result"], "post_reboot_recovery")
	}
}

func TestDoSSHPollWaitsForPamNologinWindowToClear(t *testing.T) {
	logger, captured := newCaptureLogger()
	// First the host is unreachable (reboot in progress), then it accepts the
	// connection but is still booting (pam_nologin / /run/nologin present), and
	// only the final probe reports boot completion.
	sshRunner := &scriptedSSHRunner{
		t: t,
		responses: []scriptedSSHResponse{{
			err: errors.New("connection refused"),
		}, {
			result: &remote.CommandResult{
				ExitCode: 1,
				Stderr:   "** WARNING: connection is not using a post-quantum key exchange algorithm.\nSystem is booting up. Unprivileged users are not permitted to log in yet. Please come back later. For technical details, see pam_nologin(8).",
			},
		}, {
			result: &remote.CommandResult{ExitCode: 0},
		}},
	}
	orch, s, exec := newSSHOrchestrator(t, sshRunner, logger)
	exec.CurrentState = domain.StateRebooting
	if err := s.UpdateExecution(context.Background(), exec); err != nil {
		t.Fatalf("update execution: %v", err)
	}

	if err := orch.doSSHPoll(context.Background(), exec); err != nil {
		t.Fatalf("doSSHPoll() error = %v", err)
	}
	if len(sshRunner.requests) != 3 {
		t.Fatalf("sshRunner requests = %d, want 3", len(sshRunner.requests))
	}

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateHostReachable {
		t.Fatalf("execution state = %s, want %s", updated.CurrentState, domain.StateHostReachable)
	}

	progressLogs := captured.findAll("wait.progress")
	if len(progressLogs) < 2 {
		t.Fatalf("wait.progress logs = %d, want at least 2", len(progressLogs))
	}
	if progressLogs[0].Attrs["probe_result"] != "reboot_progress_observed" {
		t.Fatalf("first wait.progress probe_result = %q, want %q", progressLogs[0].Attrs["probe_result"], "reboot_progress_observed")
	}
	if progressLogs[1].Attrs["probe_result"] != "connected_still_booting" {
		t.Fatalf("second wait.progress probe_result = %q, want %q", progressLogs[1].Attrs["probe_result"], "connected_still_booting")
	}
}

func TestDoSSHPollFailsWhenHostNeverBecomesUnreachable(t *testing.T) {
	logger, captured := newCaptureLogger()
	sshRunner := &scriptedSSHRunner{
		t: t,
		responses: []scriptedSSHResponse{{
			result: &remote.CommandResult{ExitCode: 0},
		}, {
			result: &remote.CommandResult{ExitCode: 0},
		}, {
			result: &remote.CommandResult{ExitCode: 0},
		}, {
			result: &remote.CommandResult{ExitCode: 0},
		}},
		defaultResp: &scriptedSSHResponse{result: &remote.CommandResult{ExitCode: 0}},
	}
	orch, s, exec := newSSHOrchestrator(t, sshRunner, logger)
	exec.CurrentState = domain.StateRebooting
	if err := s.UpdateExecution(context.Background(), exec); err != nil {
		t.Fatalf("update execution: %v", err)
	}

	if err := orch.doSSHPoll(context.Background(), exec); err != nil {
		t.Fatalf("doSSHPoll() error = %v", err)
	}

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateFailedManualRecovery {
		t.Fatalf("execution state = %s, want %s", updated.CurrentState, domain.StateFailedManualRecovery)
	}
	if updated.FinalErrorCode != "ssh_poll_timeout" {
		t.Fatalf("final error code = %q, want %q", updated.FinalErrorCode, "ssh_poll_timeout")
	}

	progressLogs := captured.findAll("wait.progress")
	if len(progressLogs) == 0 {
		t.Fatal("expected wait.progress logs, got none")
	}
	for i, record := range progressLogs {
		if record.Attrs["probe_result"] != "ignored_early_success" {
			t.Fatalf("wait.progress %d probe_result = %q, want %q", i+1, record.Attrs["probe_result"], "ignored_early_success")
		}
	}
}

func TestDoSSHPollFailsWhenHostDoesNotReturnAfterGoingDown(t *testing.T) {
	logger, captured := newCaptureLogger()
	sshRunner := &scriptedSSHRunner{
		t: t,
		responses: []scriptedSSHResponse{{
			err: errors.New("connection refused"),
		}, {
			err: errors.New("connection refused"),
		}, {
			err: errors.New("connection refused"),
		}, {
			err: errors.New("connection refused"),
		}},
		defaultResp: &scriptedSSHResponse{err: errors.New("connection refused")},
	}
	orch, s, exec := newSSHOrchestrator(t, sshRunner, logger)
	exec.CurrentState = domain.StateRebooting
	if err := s.UpdateExecution(context.Background(), exec); err != nil {
		t.Fatalf("update execution: %v", err)
	}

	if err := orch.doSSHPoll(context.Background(), exec); err != nil {
		t.Fatalf("doSSHPoll() error = %v", err)
	}

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateFailedManualRecovery {
		t.Fatalf("execution state = %s, want %s", updated.CurrentState, domain.StateFailedManualRecovery)
	}
	if updated.FinalErrorCode != "ssh_poll_timeout" {
		t.Fatalf("final error code = %q, want %q", updated.FinalErrorCode, "ssh_poll_timeout")
	}

	progressLogs := captured.findAll("wait.progress")
	if len(progressLogs) < 2 {
		t.Fatalf("wait.progress logs = %d, want at least 2", len(progressLogs))
	}
	if progressLogs[0].Attrs["probe_result"] != "reboot_progress_observed" {
		t.Fatalf("first wait.progress probe_result = %q, want %q", progressLogs[0].Attrs["probe_result"], "reboot_progress_observed")
	}
	for i, record := range progressLogs[1:] {
		if record.Attrs["probe_result"] != "still_unreachable" {
			t.Fatalf("wait.progress %d probe_result = %q, want %q", i+2, record.Attrs["probe_result"], "still_unreachable")
		}
	}
}

func TestDoSSHPollTreatsNonZeroProbeExitAsUnreachable(t *testing.T) {
	logger, captured := newCaptureLogger()
	sshRunner := &scriptedSSHRunner{
		t: t,
		responses: []scriptedSSHResponse{{
			result: &remote.CommandResult{ExitCode: 255, Stderr: "Connection refused"},
		}, {
			result: &remote.CommandResult{ExitCode: 0},
		}},
	}
	orch, s, exec := newSSHOrchestrator(t, sshRunner, logger)
	exec.CurrentState = domain.StateRebooting
	if err := s.UpdateExecution(context.Background(), exec); err != nil {
		t.Fatalf("update execution: %v", err)
	}

	if err := orch.doSSHPoll(context.Background(), exec); err != nil {
		t.Fatalf("doSSHPoll() error = %v", err)
	}

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateHostReachable {
		t.Fatalf("execution state = %s, want %s", updated.CurrentState, domain.StateHostReachable)
	}

	progressLogs := captured.findAll("wait.progress")
	if len(progressLogs) == 0 {
		t.Fatal("expected wait.progress log, got none")
	}
	if progressLogs[0].Attrs["probe_result"] != "reboot_progress_observed" {
		t.Fatalf("first wait.progress probe_result = %q, want %q", progressLogs[0].Attrs["probe_result"], "reboot_progress_observed")
	}

	satisfied := captured.find("wait.satisfied")
	if satisfied == nil {
		t.Fatal("expected wait.satisfied log, got none")
	}
	if satisfied.Attrs["probe_result"] != "post_reboot_recovery" {
		t.Fatalf("wait.satisfied probe_result = %q, want %q", satisfied.Attrs["probe_result"], "post_reboot_recovery")
	}
}
