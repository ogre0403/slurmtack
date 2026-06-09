package openstack

import (
	"context"
	"errors"
	"testing"
)

type fakeReadinessClient struct {
	computeService    *ComputeServiceStatus
	computeServiceErr error
	instances         []Instance
	instancesErr      error
	migrations        []string
	migrationsErr     error
}

func (f *fakeReadinessClient) GetComputeService(_ context.Context, _ string) (*ComputeServiceStatus, error) {
	if f.computeServiceErr != nil {
		return nil, f.computeServiceErr
	}
	return f.computeService, nil
}

func (f *fakeReadinessClient) ListInstances(_ context.Context, _ string) ([]Instance, error) {
	return f.instances, f.instancesErr
}

func (f *fakeReadinessClient) ListActiveMigrations(_ context.Context, _ string) ([]string, error) {
	return f.migrations, f.migrationsErr
}

func (f *fakeReadinessClient) DisableComputeService(_ context.Context, _, _ string) error { return nil }
func (f *fakeReadinessClient) EnableComputeService(_ context.Context, _ string) error      { return nil }

func TestEvaluateSourceReadiness_Ready(t *testing.T) {
	client := &fakeReadinessClient{
		computeService: &ComputeServiceStatus{Host: "gpu-01", Enabled: false},
	}
	result, err := EvaluateSourceReadiness(context.Background(), client, "gpu-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ready {
		t.Fatal("expected Ready=true")
	}
	if len(result.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %d", len(result.Blockers))
	}
	if result.ErrorSummary() != "" {
		t.Fatalf("expected empty summary, got %q", result.ErrorSummary())
	}
}

func TestEvaluateSourceReadiness_ResidentInstances(t *testing.T) {
	client := &fakeReadinessClient{
		computeService: &ComputeServiceStatus{Host: "gpu-01", Enabled: false},
		instances:      []Instance{{ID: "vm-1"}, {ID: "vm-2"}},
	}
	result, err := EvaluateSourceReadiness(context.Background(), client, "gpu-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Fatal("expected Ready=false")
	}
	if result.ErrorSummary() != "resident instances: 2" {
		t.Fatalf("unexpected summary: %q", result.ErrorSummary())
	}
}

func TestEvaluateSourceReadiness_ActiveMigrations(t *testing.T) {
	client := &fakeReadinessClient{
		computeService: &ComputeServiceStatus{Host: "gpu-01", Enabled: false},
		migrations:     []string{"mig-1"},
	}
	result, err := EvaluateSourceReadiness(context.Background(), client, "gpu-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Fatal("expected Ready=false")
	}
	if result.ErrorSummary() != "active migrations: 1" {
		t.Fatalf("unexpected summary: %q", result.ErrorSummary())
	}
}

func TestEvaluateSourceReadiness_MultipleBlockers(t *testing.T) {
	client := &fakeReadinessClient{
		computeService: &ComputeServiceStatus{Host: "gpu-01", Enabled: false},
		instances:      []Instance{{ID: "vm-1"}},
		migrations:     []string{"mig-1", "mig-2"},
	}
	result, err := EvaluateSourceReadiness(context.Background(), client, "gpu-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Ready {
		t.Fatal("expected Ready=false")
	}
	expected := "resident instances: 1; active migrations: 2"
	if result.ErrorSummary() != expected {
		t.Fatalf("unexpected summary: got %q, want %q", result.ErrorSummary(), expected)
	}
	if len(result.Blockers) != 2 {
		t.Fatalf("expected 2 blockers, got %d", len(result.Blockers))
	}
	if result.Blockers[0].Kind != "resident_instances" {
		t.Fatalf("first blocker kind = %q, want resident_instances", result.Blockers[0].Kind)
	}
	if result.Blockers[1].Kind != "active_migrations" {
		t.Fatalf("second blocker kind = %q, want active_migrations", result.Blockers[1].Kind)
	}
}

func TestEvaluateSourceReadiness_ComputeServiceError(t *testing.T) {
	client := &fakeReadinessClient{
		computeServiceErr: errors.New("nova unavailable"),
	}
	_, err := EvaluateSourceReadiness(context.Background(), client, "gpu-01")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, client.computeServiceErr) {
		t.Fatalf("expected wrapped nova error, got %v", err)
	}
}

func TestEvaluateSourceReadiness_InstanceListError(t *testing.T) {
	client := &fakeReadinessClient{
		computeService: &ComputeServiceStatus{Host: "gpu-01", Enabled: false},
		instancesErr:   errors.New("instance list failed"),
	}
	_, err := EvaluateSourceReadiness(context.Background(), client, "gpu-01")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEvaluateSourceReadiness_MigrationListError(t *testing.T) {
	client := &fakeReadinessClient{
		computeService: &ComputeServiceStatus{Host: "gpu-01", Enabled: false},
		migrationsErr:  errors.New("migration list failed"),
	}
	_, err := EvaluateSourceReadiness(context.Background(), client, "gpu-01")
	if err == nil {
		t.Fatal("expected error")
	}
}
