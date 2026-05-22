package orchestrator

import (
	"context"
	"log"
	"time"

	"github.com/slurmtack/slurmtack/internal/remote"
)

type ReachabilityConfig struct {
	Interval time.Duration
	Timeout  time.Duration
}

func PollSSHReachable(ctx context.Context, runner remote.Runner, host string, cfg ReachabilityConfig) error {
	deadline := time.After(cfg.Timeout)
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return ErrSSHPollTimeout
		case <-ticker.C:
			attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := runner.Execute(attemptCtx, remote.CommandRequest{
				Host:    host,
				Command: "hostname",
				Timeout: 5 * time.Second,
			})
			cancel()
			if err == nil {
				log.Printf("orchestrator: host %s is reachable", host)
				return nil
			}
			log.Printf("orchestrator: ssh poll to %s failed: %v", host, err)
		}
	}
}
