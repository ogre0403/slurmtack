package orchestrator

import (
	"context"
	"log/slog"
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

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			logger.Warn(trace.EventWaitTimeout, "component", "reachability", "host", host)
			return ErrSSHPollTimeout
		case <-ticker.C:
			attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := runner.Execute(attemptCtx, remote.CommandRequest{
				Host:        host,
				Command:     "hostname",
				ExecutionID: executionID,
				StepName:    sshProbeStepName,
				Timeout:     5 * time.Second,
			})
			cancel()
			if err == nil {
				logger.Info(trace.EventWaitSatisfied, "component", "reachability", "host", host)
				return nil
			}
			logger.Debug(trace.EventWaitProgress, "component", "reachability", "host", host, "error", err.Error())
		}
	}
}
