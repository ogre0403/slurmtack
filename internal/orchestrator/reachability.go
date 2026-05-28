package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/trace"
)

type ReachabilityConfig struct {
	Interval time.Duration
	Timeout  time.Duration
}

const sshProbeStepName = "ssh_probe"

func PollSSHReachable(ctx context.Context, runner remote.Runner, host, executionID string, cfg ReachabilityConfig, logger *slog.Logger) error {
	logger = trace.OrDefault(logger)
	logger.Info(trace.EventWaitEntered, "component", "reachability", "host", host, "wait_for", "ssh_reachability")

	deadline := time.After(cfg.Timeout)
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	rebootObserved := false

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			logger.Warn(trace.EventWaitTimeout, "component", "reachability", "host", host)
			return ErrSSHPollTimeout
		case <-ticker.C:
			attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			result, err := runner.Execute(attemptCtx, remote.CommandRequest{
				Host:        host,
				Command:     "hostname",
				ExecutionID: executionID,
				StepName:    sshProbeStepName,
				Timeout:     5 * time.Second,
			})
			cancel()
			err = classifyProbeResult(result, err)
			if err == nil {
				if !rebootObserved {
					logger.Debug(trace.EventWaitProgress,
						"component", "reachability",
						"host", host,
						"probe_phase", "waiting_for_reboot_start",
						"probe_result", "ignored_early_success",
					)
					continue
				}
				logger.Info(trace.EventWaitSatisfied,
					"component", "reachability",
					"host", host,
					"probe_phase", "waiting_for_host_return",
					"probe_result", "post_reboot_recovery",
				)
				return nil
			}
			if !rebootObserved {
				rebootObserved = true
				logger.Debug(trace.EventWaitProgress,
					"component", "reachability",
					"host", host,
					"probe_phase", "waiting_for_reboot_start",
					"probe_result", "reboot_progress_observed",
					"error", err.Error(),
				)
				continue
			}
			logger.Debug(trace.EventWaitProgress,
				"component", "reachability",
				"host", host,
				"probe_phase", "waiting_for_host_return",
				"probe_result", "still_unreachable",
				"error", err.Error(),
			)
		}
	}
}

func classifyProbeResult(result *remote.CommandResult, err error) error {
	if err != nil {
		return err
	}
	if result == nil || result.ExitCode == 0 {
		return nil
	}

	message := strings.TrimSpace(result.Stderr)
	if message == "" {
		message = strings.TrimSpace(result.Stdout)
	}
	if message == "" {
		return fmt.Errorf("ssh probe exited with code %d", result.ExitCode)
	}
	return fmt.Errorf("ssh probe exited with code %d: %s", result.ExitCode, message)
}
