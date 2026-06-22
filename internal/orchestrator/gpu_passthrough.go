package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/trace"
)

const (
	gpuPassthroughReconfigureScript = "reconfigure.sh"
	gpuPassthroughVerifyScript      = "verify.sh"
	gpuPassthroughLibScript         = "lib.sh"

	// gpuPassthroughStageTimeout bounds a single scp copy; gpuPassthroughExecTimeout
	// bounds the staged script execution. Reconfiguration rebuilds initramfs and
	// can run for a while, so the exec budget is generous.
	gpuPassthroughStageTimeout = 60 * time.Second
	gpuPassthroughExecTimeout  = 10 * time.Minute
)

// gpuPassthroughMode is the action passed to the reconfigure/verify scripts.
type gpuPassthroughMode string

const (
	gpuPassthroughEnable  gpuPassthroughMode = "enable"
	gpuPassthroughDisable gpuPassthroughMode = "disable"
)

// gpuPassthroughModeFor maps a switch direction to the passthrough action that
// the host must be reconfigured and verified against: slurm_to_openstack
// enables passthrough, openstack_to_slurm disables it.
func gpuPassthroughModeFor(direction domain.SwitchDirection) (gpuPassthroughMode, bool) {
	switch direction {
	case domain.DirectionSlurmToOpenStack:
		return gpuPassthroughEnable, true
	case domain.DirectionOpenStackToSlurm:
		return gpuPassthroughDisable, true
	}
	return "", false
}

// gpuPassthroughEnabled reports whether GPU passthrough script staging is
// configured. When disabled, the orchestrator skips staging/execution so the
// scripts can still be validated manually before integration is switched on.
func (o *Orchestrator) gpuPassthroughEnabled() bool {
	return o.cfg.GPUPassthroughScriptDir != ""
}

// remoteStagingBase returns the base remote directory scripts are copied into.
func (o *Orchestrator) remoteStagingBase() string {
	if o.cfg.RemoteStagingDir != "" {
		return o.cfg.RemoteStagingDir
	}
	return "/tmp"
}

// stageAndRunGPUScript copies a GPU passthrough script (plus its shared lib.sh)
// into an execution-scoped remote directory and executes it over SSH with the
// requested mode. Staging and execution share the configured SSH transport.
// Any staging or non-zero execution failure is returned so the caller can fail
// the workflow before proceeding.
func (o *Orchestrator) stageAndRunGPUScript(ctx context.Context, exec *domain.Execution, scriptName string, mode gpuPassthroughMode, stepName string) error {
	if o.sshRunner == nil {
		return errors.New("ssh runner not configured")
	}

	// Execution-scoped remote directory keeps concurrent executions and reruns
	// from clobbering each other's staged artifacts.
	remoteDir := path.Join(o.remoteStagingBase(), "slurmtack-gpu-passthrough", exec.ID)
	remoteScript := path.Join(remoteDir, scriptName)

	if _, err := o.sshRunner.Execute(ctx, remote.CommandRequest{
		Host:        exec.NodeName,
		Command:     "mkdir",
		Args:        []string{"-p", remoteDir},
		ExecutionID: exec.ID,
		StepName:    stepName + "_mkdir",
		Timeout:     gpuPassthroughStageTimeout,
	}); err != nil {
		return fmt.Errorf("preparing remote staging dir: %w", err)
	}

	// The reconfigure/verify scripts source lib.sh from their own directory, so
	// stage it alongside the target script.
	for _, name := range []string{gpuPassthroughLibScript, scriptName} {
		if err := o.sshRunner.Stage(ctx, remote.StageRequest{
			Host:        exec.NodeName,
			LocalPath:   filepath.Join(o.cfg.GPUPassthroughScriptDir, name),
			RemotePath:  path.Join(remoteDir, name),
			ExecutionID: exec.ID,
			StepName:    stepName + "_stage",
			Timeout:     gpuPassthroughStageTimeout,
		}); err != nil {
			return fmt.Errorf("staging %s: %w", name, err)
		}
	}

	result, err := o.sshRunner.Execute(ctx, remote.CommandRequest{
		Host:        exec.NodeName,
		Command:     "bash",
		Args:        []string{remoteScript, string(mode)},
		ExecutionID: exec.ID,
		StepName:    stepName,
		Timeout:     gpuPassthroughExecTimeout,
	})
	if err != nil {
		return fmt.Errorf("executing %s %s: %w", scriptName, mode, err)
	}
	if result == nil {
		return fmt.Errorf("executing %s %s: empty ssh result", scriptName, mode)
	}
	if result.ExitCode != 0 {
		message := result.Stderr
		if message == "" {
			message = result.Stdout
		}
		if message == "" {
			return fmt.Errorf("%s %s failed: exit code %d", scriptName, mode, result.ExitCode)
		}
		return fmt.Errorf("%s %s failed: exit code %d: %s", scriptName, mode, result.ExitCode, message)
	}

	trace.ForExecution(o.logger, exec).Info(trace.EventActionSucceeded,
		"action", stepName,
		"script", scriptName,
		"mode", string(mode),
	)
	return nil
}

// reconfigureGPUPassthrough stages and runs reconfigure.sh in the direction's
// passthrough mode before reboot. A no-op when passthrough scripting is disabled.
func (o *Orchestrator) reconfigureGPUPassthrough(ctx context.Context, exec *domain.Execution) error {
	if !o.gpuPassthroughEnabled() {
		return nil
	}
	mode, ok := gpuPassthroughModeFor(exec.Direction)
	if !ok {
		return fmt.Errorf("no GPU passthrough mode for direction %s", exec.Direction)
	}
	return o.stageAndRunGPUScript(ctx, exec, gpuPassthroughReconfigureScript, mode, "gpu_passthrough_reconfigure")
}

// verifyGPUPassthrough stages and runs verify.sh in the direction's passthrough
// mode after reboot, before any target attach action. A no-op when passthrough
// scripting is disabled.
func (o *Orchestrator) verifyGPUPassthrough(ctx context.Context, exec *domain.Execution) error {
	if !o.gpuPassthroughEnabled() {
		return nil
	}
	mode, ok := gpuPassthroughModeFor(exec.Direction)
	if !ok {
		return fmt.Errorf("no GPU passthrough mode for direction %s", exec.Direction)
	}
	return o.stageAndRunGPUScript(ctx, exec, gpuPassthroughVerifyScript, mode, "gpu_passthrough_verify")
}
