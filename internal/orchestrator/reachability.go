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

// probeCommand and probeArgs verify that the host has finished booting rather
// than merely that sshd accepts connections. While the system is still booting
// pam_nologin rejects unprivileged logins and /run/nologin is present; the
// command exits non-zero in that window and exits 0 only once boot completes.
var (
	probeCommand = "test"
	probeArgs    = []string{"!", "-f", "/run/nologin"}
)

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
				Command:     probeCommand,
				Args:        probeArgs,
				ExecutionID: executionID,
				StepName:    sshProbeStepName,
				Timeout:     5 * time.Second,
			})
			cancel()
			booting := isBootIncomplete(result, err)
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
			// A host that accepts the connection but is still booting
			// (pam_nologin) is evidence the reboot has happened; it must not
			// satisfy reachability, so keep polling until boot completes.
			if !rebootObserved {
				rebootObserved = true
				progressResult := "reboot_progress_observed"
				if booting {
					progressResult = "connected_still_booting"
				}
				logger.Debug(trace.EventWaitProgress,
					"component", "reachability",
					"host", host,
					"probe_phase", "waiting_for_reboot_start",
					"probe_result", progressResult,
					"error", err.Error(),
				)
				continue
			}
			progressResult := "still_unreachable"
			if booting {
				progressResult = "connected_still_booting"
			}
			logger.Debug(trace.EventWaitProgress,
				"component", "reachability",
				"host", host,
				"probe_phase", "waiting_for_host_return",
				"probe_result", progressResult,
				"error", err.Error(),
			)
		}
	}
}

func classifyProbeResult(result *remote.CommandResult, err error) error {
	if err != nil {
		return err
	}
	// A boot-completion check exiting 0 means the host has left the
	// pam_nologin window. The harmless post-quantum key-exchange warning that
	// SSH emits on stderr is irrelevant once the check itself succeeds.
	if result == nil || result.ExitCode == 0 {
		return nil
	}

	message := bootMessage(result)
	if message == "" {
		return fmt.Errorf("ssh probe exited with code %d", result.ExitCode)
	}
	return fmt.Errorf("ssh probe exited with code %d: %s", result.ExitCode, message)
}

// bootMessage extracts the meaningful probe message, preferring stderr but
// dropping the post-quantum key-exchange warning banner so it neither hides the
// real cause nor reads as a failure on its own.
func bootMessage(result *remote.CommandResult) string {
	message := stripPostQuantumWarning(result.Stderr)
	if message == "" {
		message = stripPostQuantumWarning(result.Stdout)
	}
	return message
}

// stripPostQuantumWarning removes the OpenSSH post-quantum key-exchange warning
// lines, which appear on stderr of every connection regardless of success.
func stripPostQuantumWarning(s string) string {
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "post-quantum key exchange") ||
			strings.Contains(lower, "store now, decrypt later") ||
			strings.Contains(lower, "openssh.com/pq") ||
			strings.Contains(lower, "the server may need to be upgraded") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// isBootIncomplete reports whether a probe or command outcome indicates the
// target is still booting (pam_nologin rejecting unprivileged logins or the
// /run/nologin gate still present), as opposed to a hard failure.
func isBootIncomplete(result *remote.CommandResult, err error) bool {
	if err != nil {
		return messageIndicatesBooting(err.Error())
	}
	if result == nil {
		return false
	}
	return messageIndicatesBooting(result.Stderr) || messageIndicatesBooting(result.Stdout)
}

func messageIndicatesBooting(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "system is booting up") ||
		strings.Contains(lower, "not permitted to log in yet") ||
		strings.Contains(lower, "pam_nologin") ||
		strings.Contains(lower, "/run/nologin")
}
