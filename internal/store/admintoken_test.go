package store

import (
	"context"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
)

// adminTokenRenewalStore is the subset of Store exercised by these tests, so
// the same cases run against both the SQLite and in-memory implementations.
type adminTokenRenewalStore interface {
	RecordAdminTokenRenewal(ctx context.Context, renewal *domain.AdminTokenRenewal) error
	ListAdminTokenRenewals(ctx context.Context) ([]*domain.AdminTokenRenewal, error)
}

func TestRecordAdminTokenRenewal(t *testing.T) {
	stores := map[string]adminTokenRenewalStore{
		"sqlite": newTestStore(t),
		"memory": NewMemoryStore(),
	}

	for name, s := range stores {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			// Initial cache fill.
			cacheMissAt := time.Now().UTC().Truncate(time.Second)
			if err := s.RecordAdminTokenRenewal(ctx, &domain.AdminTokenRenewal{
				IssuedAt:  cacheMissAt,
				AdminUser: "root",
				LoginNode: "login-01",
				Trigger:   domain.AdminTokenRenewalTriggerCacheMiss,
			}); err != nil {
				t.Fatalf("record cache_miss: %v", err)
			}

			// Auth-failure-driven renewal.
			authFailAt := cacheMissAt.Add(time.Minute)
			if err := s.RecordAdminTokenRenewal(ctx, &domain.AdminTokenRenewal{
				IssuedAt:  authFailAt,
				AdminUser: "root",
				LoginNode: "login-01",
				Trigger:   domain.AdminTokenRenewalTriggerAuthFailure,
			}); err != nil {
				t.Fatalf("record auth_failure: %v", err)
			}

			records, err := s.ListAdminTokenRenewals(ctx)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if len(records) != 2 {
				t.Fatalf("expected 2 renewal records, got %d", len(records))
			}

			if records[0].Trigger != domain.AdminTokenRenewalTriggerCacheMiss {
				t.Errorf("record[0].Trigger = %q, want cache_miss", records[0].Trigger)
			}
			if records[1].Trigger != domain.AdminTokenRenewalTriggerAuthFailure {
				t.Errorf("record[1].Trigger = %q, want auth_failure", records[1].Trigger)
			}

			r := records[0]
			if r.AdminUser != "root" {
				t.Errorf("AdminUser = %q, want root", r.AdminUser)
			}
			if r.LoginNode != "login-01" {
				t.Errorf("LoginNode = %q, want login-01", r.LoginNode)
			}
			if !r.IssuedAt.Equal(cacheMissAt) {
				t.Errorf("IssuedAt = %v, want %v", r.IssuedAt, cacheMissAt)
			}
			if r.ID == 0 {
				t.Error("expected non-zero record ID")
			}
		})
	}
}
