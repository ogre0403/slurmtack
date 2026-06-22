package slurm

import (
	"context"
	"errors"
	"testing"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/store"
)

// fakeRunner records the commands it receives and returns scripted results.
type fakeRunner struct {
	requests []remote.CommandRequest
	results  []*remote.CommandResult
	errs     []error
	calls    int
}

func (r *fakeRunner) Execute(_ context.Context, req remote.CommandRequest) (*remote.CommandResult, error) {
	r.requests = append(r.requests, req)
	i := r.calls
	r.calls++
	var res *remote.CommandResult
	if i < len(r.results) {
		res = r.results[i]
	}
	var err error
	if i < len(r.errs) {
		err = r.errs[i]
	}
	return res, err
}

func (r *fakeRunner) Stage(_ context.Context, _ remote.StageRequest) error {
	return nil
}

func newTestProvider(t *testing.T, runner remote.Runner) (*SSHAdminTokenProvider, *store.MemoryStore) {
	t.Helper()
	s := store.NewMemoryStore()
	p := NewSSHAdminTokenProvider(SSHAdminTokenProviderConfig{
		Runner:    runner,
		Store:     s,
		AdminUser: "root",
		LoginNode: "login-01",
		Lifespan:  900,
	})
	return p, s
}

func TestParseScontrolToken(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		want    string
		wantErr bool
	}{
		{name: "bare line", out: "SLURM_JWT=abc.def.ghi\n", want: "abc.def.ghi"},
		{name: "with surrounding output", out: "some warning\nSLURM_JWT=tok123\n", want: "tok123"},
		{name: "trailing whitespace", out: "SLURM_JWT=tok456   \n", want: "tok456"},
		{name: "missing value", out: "no token here\n", wantErr: true},
		{name: "empty value", out: "SLURM_JWT=\n", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseScontrolToken(tc.out)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got token %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("token = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestProviderTokenMintsOnCacheMiss(t *testing.T) {
	runner := &fakeRunner{
		results: []*remote.CommandResult{{ExitCode: 0, Stdout: "SLURM_JWT=minted-token\n"}},
	}
	p, s := newTestProvider(t, runner)

	token, err := p.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token != "minted-token" {
		t.Fatalf("token = %q, want minted-token", token)
	}

	// Verify the scontrol command was rendered with the configured user/lifespan.
	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1", runner.calls)
	}
	req := runner.requests[0]
	if req.Host != "login-01" || req.Command != "scontrol" {
		t.Fatalf("unexpected request host/command: %+v", req)
	}
	wantArgs := []string{"token", "username=root", "lifespan=900"}
	if len(req.Args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", req.Args, wantArgs)
	}
	for i := range wantArgs {
		if req.Args[i] != wantArgs[i] {
			t.Fatalf("args = %v, want %v", req.Args, wantArgs)
		}
	}

	// One cache_miss audit record persisted, without token material.
	records, _ := s.ListAdminTokenRenewals(context.Background())
	if len(records) != 1 {
		t.Fatalf("audit records = %d, want 1", len(records))
	}
	if records[0].Trigger != domain.AdminTokenRenewalTriggerCacheMiss {
		t.Fatalf("trigger = %q, want cache_miss", records[0].Trigger)
	}
	if records[0].AdminUser != "root" || records[0].LoginNode != "login-01" {
		t.Fatalf("unexpected audit metadata: %+v", records[0])
	}
}

func TestProviderTokenReusesCache(t *testing.T) {
	runner := &fakeRunner{
		results: []*remote.CommandResult{{ExitCode: 0, Stdout: "SLURM_JWT=tok\n"}},
	}
	p, s := newTestProvider(t, runner)

	if _, err := p.Token(context.Background()); err != nil {
		t.Fatalf("first Token() error = %v", err)
	}
	if _, err := p.Token(context.Background()); err != nil {
		t.Fatalf("second Token() error = %v", err)
	}

	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1 (cache reused)", runner.calls)
	}
	records, _ := s.ListAdminTokenRenewals(context.Background())
	if len(records) != 1 {
		t.Fatalf("audit records = %d, want 1", len(records))
	}
}

func TestProviderRenewInvalidatesCacheAndRecordsAuthFailure(t *testing.T) {
	runner := &fakeRunner{
		results: []*remote.CommandResult{
			{ExitCode: 0, Stdout: "SLURM_JWT=first\n"},
			{ExitCode: 0, Stdout: "SLURM_JWT=second\n"},
		},
	}
	p, s := newTestProvider(t, runner)

	first, err := p.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	second, err := p.Renew(context.Background(), first)
	if err != nil {
		t.Fatalf("Renew() error = %v", err)
	}
	if first == second {
		t.Fatal("expected Renew to mint a different token")
	}
	if second != "second" {
		t.Fatalf("renewed token = %q, want second", second)
	}

	// Subsequent Token() reuses the freshly minted cache without re-minting.
	next, err := p.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() after renew error = %v", err)
	}
	if next != "second" {
		t.Fatalf("token = %q, want cached second", next)
	}
	if runner.calls != 2 {
		t.Fatalf("runner calls = %d, want 2", runner.calls)
	}

	records, _ := s.ListAdminTokenRenewals(context.Background())
	if len(records) != 2 {
		t.Fatalf("audit records = %d, want 2", len(records))
	}
	if records[0].Trigger != domain.AdminTokenRenewalTriggerCacheMiss {
		t.Errorf("record[0].Trigger = %q, want cache_miss", records[0].Trigger)
	}
	if records[1].Trigger != domain.AdminTokenRenewalTriggerAuthFailure {
		t.Errorf("record[1].Trigger = %q, want auth_failure", records[1].Trigger)
	}
}

func TestProviderRenewSkipsRemintWhenCacheAlreadyRotated(t *testing.T) {
	// Only one mint result is scripted: a second SSH issuance would index past
	// it, so this test fails loudly if Renew re-mints unnecessarily.
	runner := &fakeRunner{
		results: []*remote.CommandResult{{ExitCode: 0, Stdout: "SLURM_JWT=fresh\n"}},
	}
	s := store.NewMemoryStore()
	p := NewSSHAdminTokenProvider(SSHAdminTokenProviderConfig{
		Runner:         runner,
		Store:          s,
		AdminUser:      "root",
		LoginNode:      "login-01",
		BootstrapToken: "stale",
	})

	// Two concurrent admin requests both used the seeded "stale" token and both
	// got an auth failure. The first renews and mints "fresh".
	first, err := p.Renew(context.Background(), "stale")
	if err != nil {
		t.Fatalf("first Renew() error = %v", err)
	}
	if first != "fresh" {
		t.Fatalf("first renew token = %q, want fresh", first)
	}

	// The second caller still holds "stale". The cache already rotated to
	// "fresh", so Renew must return that without a second SSH mint.
	second, err := p.Renew(context.Background(), "stale")
	if err != nil {
		t.Fatalf("second Renew() error = %v", err)
	}
	if second != "fresh" {
		t.Fatalf("second renew token = %q, want fresh", second)
	}

	if runner.calls != 1 {
		t.Fatalf("runner calls = %d, want 1 (no duplicate SSH mint)", runner.calls)
	}
	records, _ := s.ListAdminTokenRenewals(context.Background())
	if len(records) != 1 {
		t.Fatalf("audit records = %d, want 1 (no duplicate audit)", len(records))
	}
}

func TestProviderMintFailsOnNonZeroExit(t *testing.T) {
	runner := &fakeRunner{
		results: []*remote.CommandResult{{ExitCode: 1, Stderr: "permission denied"}},
	}
	p, s := newTestProvider(t, runner)

	if _, err := p.Token(context.Background()); err == nil {
		t.Fatal("expected error on non-zero scontrol exit")
	}
	// No audit record should be persisted for a failed issuance.
	records, _ := s.ListAdminTokenRenewals(context.Background())
	if len(records) != 0 {
		t.Fatalf("audit records = %d, want 0 on failure", len(records))
	}
}

func TestProviderMintFailsOnRunnerError(t *testing.T) {
	runner := &fakeRunner{errs: []error{errors.New("ssh dial failed")}}
	p, _ := newTestProvider(t, runner)

	if _, err := p.Token(context.Background()); err == nil {
		t.Fatal("expected error when runner fails")
	}
}

func TestProviderSeedsFromBootstrapToken(t *testing.T) {
	runner := &fakeRunner{}
	s := store.NewMemoryStore()
	p := NewSSHAdminTokenProvider(SSHAdminTokenProviderConfig{
		Runner:         runner,
		Store:          s,
		AdminUser:      "root",
		LoginNode:      "login-01",
		BootstrapToken: "bootstrap",
	})

	token, err := p.Token(context.Background())
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}
	if token != "bootstrap" {
		t.Fatalf("token = %q, want bootstrap", token)
	}
	if runner.calls != 0 {
		t.Fatalf("runner calls = %d, want 0 (seeded cache)", runner.calls)
	}
	// Seeding from a static token is not an SSH issuance, so no audit record.
	records, _ := s.ListAdminTokenRenewals(context.Background())
	if len(records) != 0 {
		t.Fatalf("audit records = %d, want 0 for bootstrap seed", len(records))
	}
}
