package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/openstack"
	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

var errStage = errors.New("scp staging failed on gpu-node-01: connection refused")

// newGPUOrchestrator builds an orchestrator wired with the given SSH runner and
// a GPU passthrough script directory populated with stub script files, plus an
// execution parked in the requested state/direction.
func newGPUOrchestrator(t *testing.T, sshRunner remote.Runner, slurmClient slurm.Client, osClient *fakeOpenStackClient, state domain.SwitchState, direction domain.SwitchDirection) (*Orchestrator, store.Store, *domain.Execution) {
	t.Helper()

	scriptDir := t.TempDir()
	for _, name := range []string{"lib.sh", "reconfigure.sh", "verify.sh"} {
		if err := os.WriteFile(filepath.Join(scriptDir, name), []byte("#!/usr/bin/env bash\n"), 0o755); err != nil {
			t.Fatalf("write stub script %s: %v", name, err)
		}
	}

	s := store.NewMemoryStore()
	ctx := context.Background()

	desired, previous := domain.OwnerSlurm, domain.OwnerOpenStack
	if direction == domain.DirectionSlurmToOpenStack {
		desired, previous = domain.OwnerOpenStack, domain.OwnerSlurm
	}
	exec := &domain.Execution{
		ID:            "gpu-exec-1",
		NodeName:      "gpu-node-01",
		Direction:     direction,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  state,
		DesiredOwner:  desired,
		PreviousOwner: previous,
		StateVersion:  1,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	logger, _ := newCaptureLogger()
	runner := engine.NewRunner(s, logger)
	var oc openstack.Client
	if osClient != nil {
		oc = osClient
	}
	orch := New(s, runner, sshRunner, slurmClient, oc, Config{
		SSHPollInterval:         time.Millisecond,
		SSHPollTimeout:          50 * time.Millisecond,
		GPUPassthroughScriptDir: scriptDir,
	}, logger)

	return orch, s, exec
}

// stagedScripts returns the remote-path basenames of every staged file.
func stagedScripts(reqs []remote.StageRequest) []string {
	names := make([]string, 0, len(reqs))
	for _, r := range reqs {
		names = append(names, filepath.Base(r.RemotePath))
	}
	return names
}

// execScriptCalls returns the (script-basename, mode) pairs of bash invocations
// against staged GPU passthrough scripts, ignoring the mkdir staging command.
func execScriptCalls(reqs []remote.CommandRequest) [][2]string {
	var calls [][2]string
	for _, r := range reqs {
		if r.Command != "bash" || len(r.Args) != 2 {
			continue
		}
		calls = append(calls, [2]string{filepath.Base(r.Args[0]), r.Args[1]})
	}
	return calls
}

func TestReconfigureStagesAndRunsEnableBeforeReboot(t *testing.T) {
	ssh := &recordingSSHRunner{}
	orch, s, exec := newGPUOrchestrator(t, ssh, nil, nil, domain.StateSourceDetached, domain.DirectionSlurmToOpenStack)
	// slurmd stop/disable run first for s2o; give them success (default exit 0).

	if err := orch.doReconfigure(context.Background(), exec); err != nil {
		t.Fatalf("doReconfigure() error = %v", err)
	}

	// lib.sh + reconfigure.sh staged.
	staged := stagedScripts(ssh.stageRequests)
	if len(staged) != 2 || staged[0] != "lib.sh" || staged[1] != "reconfigure.sh" {
		t.Fatalf("staged scripts = %v, want [lib.sh reconfigure.sh]", staged)
	}

	calls := execScriptCalls(ssh.requests)
	if len(calls) != 1 || calls[0] != [2]string{"reconfigure.sh", "enable"} {
		t.Fatalf("script exec calls = %v, want [[reconfigure.sh enable]]", calls)
	}

	// scp must happen before the script is executed.
	if len(ssh.stageRequests) == 0 {
		t.Fatal("expected stage requests before execution")
	}

	updated, err := s.GetExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if updated.CurrentState != domain.StateHostReconfiguring {
		t.Fatalf("state = %s, want %s", updated.CurrentState, domain.StateHostReconfiguring)
	}
}

func TestReconfigureStagesAndRunsDisableBeforeReboot(t *testing.T) {
	ssh := &recordingSSHRunner{}
	orch, _, exec := newGPUOrchestrator(t, ssh, nil, nil, domain.StateSourceDetached, domain.DirectionOpenStackToSlurm)

	if err := orch.doReconfigure(context.Background(), exec); err != nil {
		t.Fatalf("doReconfigure() error = %v", err)
	}

	calls := execScriptCalls(ssh.requests)
	if len(calls) != 1 || calls[0] != [2]string{"reconfigure.sh", "disable"} {
		t.Fatalf("script exec calls = %v, want [[reconfigure.sh disable]]", calls)
	}
}

func TestReconfigureStagingFailureBlocksReboot(t *testing.T) {
	ssh := &recordingSSHRunner{stageErr: errStage}
	orch, s, exec := newGPUOrchestrator(t, ssh, nil, nil, domain.StateSourceDetached, domain.DirectionOpenStackToSlurm)

	err := orch.doReconfigure(context.Background(), exec)
	if err == nil {
		t.Fatal("doReconfigure() error = nil, want staging failure")
	}
	if !strings.Contains(err.Error(), "staging") {
		t.Fatalf("error = %q, want staging mention", err.Error())
	}

	updated, getErr := s.GetExecution(context.Background(), exec.ID)
	if getErr != nil {
		t.Fatalf("get execution: %v", getErr)
	}
	if updated.CurrentState == domain.StateRebooting || updated.CurrentState == domain.StateHostReconfiguring {
		t.Fatalf("state = %s, must not advance toward reboot", updated.CurrentState)
	}
}

func TestReconfigureScriptFailureBlocksReboot(t *testing.T) {
	ssh := &scriptedSSHRunner{
		t: t,
		// mkdir succeeds, reconfigure.sh exits non-zero.
		responses: []scriptedSSHResponse{
			{result: &remote.CommandResult{ExitCode: 0}},
			{result: &remote.CommandResult{ExitCode: 3, Stderr: "no NVIDIA"}},
		},
	}
	orch, s, exec := newGPUOrchestrator(t, ssh, nil, nil, domain.StateSourceDetached, domain.DirectionOpenStackToSlurm)

	err := orch.doReconfigure(context.Background(), exec)
	if err == nil {
		t.Fatal("doReconfigure() error = nil, want script failure")
	}
	if !strings.Contains(err.Error(), "reconfigure.sh disable failed") || !strings.Contains(err.Error(), "no NVIDIA") {
		t.Fatalf("error = %q, want reconfigure.sh failure with stderr", err.Error())
	}

	updated, getErr := s.GetExecution(context.Background(), exec.ID)
	if getErr != nil {
		t.Fatalf("get execution: %v", getErr)
	}
	if updated.CurrentState == domain.StateRebooting {
		t.Fatalf("state = %s, must not reach rebooting", updated.CurrentState)
	}
}

func TestVerifyStagesAndRunsEnableBeforeOpenStackAttach(t *testing.T) {
	ssh := &recordingSSHRunner{}
	osClient := &fakeOpenStackClient{}
	orch, _, exec := newGPUOrchestrator(t, ssh, nil, osClient, domain.StateHostReachable, domain.DirectionSlurmToOpenStack)

	if err := orch.doAttach(context.Background(), exec); err != nil {
		t.Fatalf("doAttach() error = %v", err)
	}

	calls := execScriptCalls(ssh.requests)
	if len(calls) != 1 || calls[0] != [2]string{"verify.sh", "enable"} {
		t.Fatalf("script exec calls = %v, want [[verify.sh enable]]", calls)
	}
	if osClient.enableComputeCalls != 1 {
		t.Fatalf("EnableComputeService calls = %d, want 1", osClient.enableComputeCalls)
	}
}

func TestVerifyStagesAndRunsDisableBeforeSlurmAttach(t *testing.T) {
	ssh := &recordingSSHRunner{}
	client := &attachTestSlurmClient{nodeState: &slurm.NodeState{NodeName: "gpu-node-01", State: "idle"}}
	orch, _, exec := newGPUOrchestrator(t, ssh, client, nil, domain.StateHostReachable, domain.DirectionOpenStackToSlurm)

	if err := orch.doAttach(context.Background(), exec); err != nil {
		t.Fatalf("doAttach() error = %v", err)
	}

	// verify.sh disable must run before slurmd enable/start commands.
	calls := execScriptCalls(ssh.requests)
	if len(calls) != 1 || calls[0] != [2]string{"verify.sh", "disable"} {
		t.Fatalf("script exec calls = %v, want [[verify.sh disable]]", calls)
	}

	// Confirm ordering: the verify bash call precedes any systemctl slurmd call.
	verifyIdx, slurmdIdx := -1, -1
	for i, r := range ssh.requests {
		if r.Command == "bash" && len(r.Args) == 2 && filepath.Base(r.Args[0]) == "verify.sh" {
			verifyIdx = i
		}
		if r.Command == "systemctl" && slurmdIdx == -1 {
			slurmdIdx = i
		}
	}
	if verifyIdx == -1 || slurmdIdx == -1 {
		t.Fatalf("expected both verify and systemctl calls, got verifyIdx=%d slurmdIdx=%d", verifyIdx, slurmdIdx)
	}
	if verifyIdx > slurmdIdx {
		t.Fatalf("verify.sh (idx %d) must run before slurmd systemctl (idx %d)", verifyIdx, slurmdIdx)
	}
}

// TestVerifyDisableFailureBlocksAttach covers the disabled-state verification
// contract: a leftover vfio-pci binding (script exit non-zero) fails the
// workflow before any Slurm attach action runs.
func TestVerifyDisableFailureBlocksAttach(t *testing.T) {
	ssh := &scriptedSSHRunner{
		t: t,
		responses: []scriptedSSHResponse{
			{result: &remote.CommandResult{ExitCode: 0}}, // mkdir
			{result: &remote.CommandResult{ExitCode: 1, Stderr: "NVIDIA device(s) still bound to vfio-pci"}},
		},
	}
	client := &attachTestSlurmClient{nodeState: &slurm.NodeState{NodeName: "gpu-node-01", State: "idle"}}
	orch, s, exec := newGPUOrchestrator(t, ssh, client, nil, domain.StateHostReachable, domain.DirectionOpenStackToSlurm)

	err := orch.doAttach(context.Background(), exec)
	if err == nil {
		t.Fatal("doAttach() error = nil, want verify failure")
	}
	if !strings.Contains(err.Error(), "verify.sh disable failed") || !strings.Contains(err.Error(), "still bound to vfio-pci") {
		t.Fatalf("error = %q, want verify.sh disable failure with vfio binding", err.Error())
	}

	// No slurmd restore commands should have run.
	for _, r := range ssh.requests {
		if r.Command == "systemctl" {
			t.Fatalf("systemctl %v ran despite failed disable verification", r.Args)
		}
	}

	updated, getErr := s.GetExecution(context.Background(), exec.ID)
	if getErr != nil {
		t.Fatalf("get execution: %v", getErr)
	}
	if updated.CurrentState == domain.StateTargetAttaching {
		t.Fatalf("state = %s, must not reach target_attaching", updated.CurrentState)
	}
}

func TestVerifyEnableFailureBlocksOpenStackAttach(t *testing.T) {
	ssh := &scriptedSSHRunner{
		t: t,
		responses: []scriptedSSHResponse{
			{result: &remote.CommandResult{ExitCode: 0}}, // mkdir
			{result: &remote.CommandResult{ExitCode: 1, Stderr: "only 0/1 NVIDIA devices bound to vfio-pci"}},
		},
	}
	osClient := &fakeOpenStackClient{}
	orch, _, exec := newGPUOrchestrator(t, ssh, nil, osClient, domain.StateHostReachable, domain.DirectionSlurmToOpenStack)

	err := orch.doAttach(context.Background(), exec)
	if err == nil {
		t.Fatal("doAttach() error = nil, want verify failure")
	}
	if osClient.enableComputeCalls != 0 {
		t.Fatalf("EnableComputeService calls = %d, want 0 (attach must be blocked)", osClient.enableComputeCalls)
	}
}

// TestFakeBundleUsedAsScriptDir confirms that GPU_PASSTHROUGH_SCRIPT_DIR can
// point at the fake bundle without adding any orchestration-specific branching:
// the orchestrator stages the same files and calls the same script names
// regardless of which compatible bundle is selected.
func TestFakeBundleUsedAsScriptDir(t *testing.T) {
	fakeBundleDir := "../../scripts/fake-passthrough"
	if _, err := os.Stat(fakeBundleDir); err != nil {
		t.Skipf("fake bundle dir %s not found: %v", fakeBundleDir, err)
	}

	ssh := &recordingSSHRunner{}
	s := store.NewMemoryStore()
	ctx := context.Background()

	exec := &domain.Execution{
		ID:            "fake-exec-1",
		NodeName:      "test-node-01",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateSourceDetached,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  1,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	logger, _ := newCaptureLogger()
	runner := engine.NewRunner(s, logger)
	orch := New(s, runner, ssh, nil, nil, Config{
		SSHPollInterval:         time.Millisecond,
		SSHPollTimeout:          50 * time.Millisecond,
		GPUPassthroughScriptDir: fakeBundleDir,
	}, logger)

	if err := orch.reconfigureGPUPassthrough(ctx, exec); err != nil {
		t.Fatalf("reconfigureGPUPassthrough() with fake bundle error = %v", err)
	}

	staged := stagedScripts(ssh.stageRequests)
	if len(staged) != 2 || staged[0] != "lib.sh" || staged[1] != "reconfigure.sh" {
		t.Fatalf("staged scripts = %v, want [lib.sh reconfigure.sh]", staged)
	}

	calls := execScriptCalls(ssh.requests)
	if len(calls) != 1 || calls[0] != [2]string{"reconfigure.sh", "enable"} {
		t.Fatalf("script exec calls = %v, want [[reconfigure.sh enable]]", calls)
	}
}

// TestFakeBundleDirectionMappingPreserved confirms that the slurm_to_openstack →
// enable and openstack_to_slurm → disable direction mapping is unchanged when the
// fake bundle is selected: the orchestrator derives the mode from the execution
// direction, not from the bundle type.
func TestFakeBundleDirectionMappingPreserved(t *testing.T) {
	fakeBundleDir := "../../scripts/fake-passthrough"
	if _, err := os.Stat(fakeBundleDir); err != nil {
		t.Skipf("fake bundle dir %s not found: %v", fakeBundleDir, err)
	}

	tests := []struct {
		name      string
		direction domain.SwitchDirection
		wantMode  string
	}{
		{"slurm_to_openstack reconfigure runs enable", domain.DirectionSlurmToOpenStack, "enable"},
		{"openstack_to_slurm reconfigure runs disable", domain.DirectionOpenStackToSlurm, "disable"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ssh := &recordingSSHRunner{}
			s := store.NewMemoryStore()
			ctx := context.Background()

			desired, previous := domain.OwnerOpenStack, domain.OwnerSlurm
			if tc.direction == domain.DirectionOpenStackToSlurm {
				desired, previous = domain.OwnerSlurm, domain.OwnerOpenStack
			}
			exec := &domain.Execution{
				ID:            "dir-test-" + tc.wantMode,
				NodeName:      "test-node-01",
				Direction:     tc.direction,
				RequestedBy:   "test",
				RequestedAt:   time.Now(),
				CurrentState:  domain.StateSourceDetached,
				DesiredOwner:  desired,
				PreviousOwner: previous,
				StateVersion:  1,
				OverallStatus: domain.OverallStatusActive,
			}
			if err := s.CreateExecution(ctx, exec); err != nil {
				t.Fatalf("create execution: %v", err)
			}

			logger, _ := newCaptureLogger()
			runner := engine.NewRunner(s, logger)
			orch := New(s, runner, ssh, nil, nil, Config{
				SSHPollInterval:         time.Millisecond,
				SSHPollTimeout:          50 * time.Millisecond,
				GPUPassthroughScriptDir: fakeBundleDir,
			}, logger)

			if err := orch.reconfigureGPUPassthrough(ctx, exec); err != nil {
				t.Fatalf("reconfigureGPUPassthrough() error = %v", err)
			}

			calls := execScriptCalls(ssh.requests)
			if len(calls) != 1 || calls[0] != [2]string{"reconfigure.sh", tc.wantMode} {
				t.Fatalf("script exec calls = %v, want [[reconfigure.sh %s]]", calls, tc.wantMode)
			}
		})
	}
}

// TestGPUPassthroughDisabledWhenScriptDirUnset proves the integration is a no-op
// when no script directory is configured, preserving the pre-integration path.
func TestGPUPassthroughDisabledWhenScriptDirUnset(t *testing.T) {
	ssh := &recordingSSHRunner{}
	s := store.NewMemoryStore()
	ctx := context.Background()
	exec := newHostReachableExecution(domain.DirectionSlurmToOpenStack)
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}
	logger, _ := newCaptureLogger()
	runner := engine.NewRunner(s, logger)
	osClient := &fakeOpenStackClient{}
	orch := New(s, runner, ssh, nil, osClient, Config{}, logger)

	if err := orch.doAttach(ctx, exec); err != nil {
		t.Fatalf("doAttach() error = %v", err)
	}
	if len(ssh.stageRequests) != 0 {
		t.Fatalf("stage requests = %d, want 0 when script dir unset", len(ssh.stageRequests))
	}
	if calls := execScriptCalls(ssh.requests); len(calls) != 0 {
		t.Fatalf("script exec calls = %v, want none", calls)
	}
}
