package slurm

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

// adminTokenProvider resolves the Slurm admin token for admin-authenticated
// requests. Token returns a cached token or mints one on a cache miss; Renew
// always invalidates the cache and mints a fresh token after an auth failure.
type adminTokenProvider interface {
	Token(ctx context.Context) (string, error)
	Renew(ctx context.Context, staleToken string) (string, error)
}

// SSHAdminTokenProvider mints short-lived Slurm admin JWTs by running
// `scontrol token` on a configured login node over SSH. It keeps a single
// in-memory cached token and records one datastore audit entry per issuance.
type SSHAdminTokenProvider struct {
	runner    remote.Runner
	store     store.Store
	adminUser string
	loginNode string
	lifespan  int
	logger    *slog.Logger
	now       func() time.Time

	mu     sync.Mutex
	cached string
}

// SSHAdminTokenProviderConfig configures an SSHAdminTokenProvider.
type SSHAdminTokenProviderConfig struct {
	Runner    remote.Runner
	Store     store.Store
	AdminUser string
	LoginNode string
	Lifespan  int
	// BootstrapToken optionally seeds the cache from a configured static admin
	// token so the first admin request does not require an SSH round trip.
	BootstrapToken string
	Logger         *slog.Logger
}

func NewSSHAdminTokenProvider(cfg SSHAdminTokenProviderConfig) *SSHAdminTokenProvider {
	lifespan := cfg.Lifespan
	if lifespan <= 0 {
		lifespan = 600
	}
	return &SSHAdminTokenProvider{
		runner:    cfg.Runner,
		store:     cfg.Store,
		adminUser: cfg.AdminUser,
		loginNode: cfg.LoginNode,
		lifespan:  lifespan,
		logger:    trace.OrDefault(cfg.Logger).With("component", "slurm_admin_token"),
		now:       time.Now,
		cached:    cfg.BootstrapToken,
	}
}

func (p *SSHAdminTokenProvider) Token(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cached != "" {
		return p.cached, nil
	}
	return p.mintLocked(ctx, domain.AdminTokenRenewalTriggerCacheMiss)
}

// Renew invalidates the cached admin token and mints a fresh one. staleToken is
// the token the caller just used and got rejected; if the cache no longer holds
// it, a concurrent caller already renewed, so Renew returns that fresh token
// without minting again. This collapses the duplicate SSH issuance that occurs
// when concurrent admin requests all fail auth against the same expired token.
func (p *SSHAdminTokenProvider) Renew(ctx context.Context, staleToken string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cached != "" && p.cached != staleToken {
		return p.cached, nil
	}
	p.cached = ""
	return p.mintLocked(ctx, domain.AdminTokenRenewalTriggerAuthFailure)
}

// mintLocked issues a fresh admin token over SSH and records the audit entry.
// Callers must hold p.mu.
func (p *SSHAdminTokenProvider) mintLocked(ctx context.Context, trigger domain.AdminTokenRenewalTrigger) (string, error) {
	if p.runner == nil {
		return "", fmt.Errorf("slurm admin token provider has no SSH runner configured")
	}

	result, err := p.runner.Execute(ctx, remote.CommandRequest{
		Host:    p.loginNode,
		Command: "scontrol",
		Args:    []string{"token", "username=" + p.adminUser, "lifespan=" + strconv.Itoa(p.lifespan)},
	})
	if err != nil {
		return "", fmt.Errorf("minting slurm admin token over ssh: %w", err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("minting slurm admin token over ssh: scontrol exited %d: %s", result.ExitCode, strings.TrimSpace(result.Stderr))
	}

	token, err := parseScontrolToken(result.Stdout)
	if err != nil {
		return "", err
	}

	if p.store != nil {
		renewal := &domain.AdminTokenRenewal{
			IssuedAt:  p.now().UTC(),
			AdminUser: p.adminUser,
			LoginNode: p.loginNode,
			Trigger:   trigger,
		}
		if err := p.store.RecordAdminTokenRenewal(ctx, renewal); err != nil {
			return "", fmt.Errorf("recording admin token renewal audit: %w", err)
		}
	}

	p.cached = token
	p.logger.Info("slurm.admin_token.issued",
		"admin_user", p.adminUser,
		"login_node", p.loginNode,
		"trigger", string(trigger),
		"lifespan_seconds", p.lifespan,
	)
	return token, nil
}

// parseScontrolToken extracts the JWT from `scontrol token` output, which emits
// a line of the form `SLURM_JWT=<token>`.
func parseScontrolToken(out string) (string, error) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "SLURM_JWT="); ok {
			token := strings.TrimSpace(rest)
			if token == "" {
				return "", fmt.Errorf("parsing scontrol token output: empty SLURM_JWT value")
			}
			return token, nil
		}
	}
	return "", fmt.Errorf("parsing scontrol token output: no SLURM_JWT value found")
}
