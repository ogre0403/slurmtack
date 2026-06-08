package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/service"
	"github.com/slurmtack/slurmtack/internal/store"
)

func setupHistoryServer(t *testing.T) (*Server, *store.SQLiteStore) {
	t.Helper()
	sqlStore, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { sqlStore.Close() })

	svc := service.NewSwitchService(sqlStore, nil)
	srv := NewServer(":0", sqlStore, svc, nil, nil, WithJWTAuth(testJWTManager, nil))
	return srv, sqlStore
}

func seedExecutions(t *testing.T, s *store.SQLiteStore) {
	t.Helper()
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	execs := []*domain.Execution{
		{ID: "e1", NodeName: "gpu-01", Direction: domain.DirectionOpenStackToSlurm, OverallStatus: domain.OverallStatusSucceeded, CurrentState: domain.StateCompleted, RequestedAt: base, RequestedBy: "user1", DesiredOwner: domain.OwnerSlurm, PreviousOwner: domain.OwnerOpenStack},
		{ID: "e2", NodeName: "gpu-01", Direction: domain.DirectionSlurmToOpenStack, OverallStatus: domain.OverallStatusFailed, CurrentState: domain.StateFailedNonDestructive, RequestedAt: base.Add(time.Hour), RequestedBy: "user2", DesiredOwner: domain.OwnerOpenStack, PreviousOwner: domain.OwnerSlurm},
		{ID: "e3", NodeName: "gpu-02", Direction: domain.DirectionOpenStackToSlurm, OverallStatus: domain.OverallStatusActive, CurrentState: domain.StateLocked, RequestedAt: base.Add(2 * time.Hour), RequestedBy: "user1", DesiredOwner: domain.OwnerSlurm, PreviousOwner: domain.OwnerOpenStack},
		{ID: "e4", NodeName: "gpu-02", Direction: domain.DirectionSlurmToOpenStack, OverallStatus: domain.OverallStatusSucceeded, CurrentState: domain.StateCompleted, RequestedAt: base.Add(3 * time.Hour), RequestedBy: "user1", DesiredOwner: domain.OwnerOpenStack, PreviousOwner: domain.OwnerSlurm},
	}
	for _, e := range execs {
		if err := s.CreateExecution(context.Background(), e); err != nil {
			t.Fatalf("seed exec: %v", err)
		}
	}
}

func doAuthGet(srv *Server, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", path, nil)
	token, _ := testJWTManager.Generate("test-operator")
	req.Header.Set("Authorization", "Bearer "+token)
	srv.Engine().ServeHTTP(w, req)
	return w
}

func TestList_DirectionFilter(t *testing.T) {
	srv, s := setupHistoryServer(t)
	seedExecutions(t, s)

	w := doAuthGet(srv, "/v1/switches?direction=openstack_to_slurm")
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	var results []ExecutionStatus
	json.Unmarshal(w.Body.Bytes(), &results)
	for _, r := range results {
		if r.Direction != "openstack_to_slurm" {
			t.Errorf("unexpected direction: %s", r.Direction)
		}
	}
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

func TestList_LimitFilter(t *testing.T) {
	srv, s := setupHistoryServer(t)
	seedExecutions(t, s)

	w := doAuthGet(srv, "/v1/switches?limit=2")
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	var results []ExecutionStatus
	json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

func TestList_BeforeFilter(t *testing.T) {
	srv, s := setupHistoryServer(t)
	seedExecutions(t, s)

	w := doAuthGet(srv, "/v1/switches?before=2026-06-01T11:30:00Z")
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	var results []ExecutionStatus
	json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) != 2 {
		t.Errorf("expected 2 (e1 at 10:00, e2 at 11:00), got %d", len(results))
	}
}

func TestList_NewestFirst(t *testing.T) {
	srv, s := setupHistoryServer(t)
	seedExecutions(t, s)

	w := doAuthGet(srv, "/v1/switches")
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	var results []ExecutionStatus
	json.Unmarshal(w.Body.Bytes(), &results)
	if len(results) < 2 {
		t.Fatalf("too few results: %d", len(results))
	}
	if results[0].RequestedAt.Before(results[1].RequestedAt) {
		t.Error("results not in newest-first order")
	}
}

func TestSteps_ReturnsOrderedSteps(t *testing.T) {
	srv, s := setupHistoryServer(t)
	exec := &domain.Execution{
		ID: "step-exec", NodeName: "gpu-01", Direction: domain.DirectionOpenStackToSlurm,
		OverallStatus: domain.OverallStatusActive, CurrentState: domain.StateLocked,
		RequestedAt: time.Now(), RequestedBy: "test",
		DesiredOwner: domain.OwnerSlurm, PreviousOwner: domain.OwnerOpenStack,
	}
	s.CreateExecution(context.Background(), exec)

	now := time.Now()
	steps := []*domain.StepRecord{
		{ExecutionID: "step-exec", StepName: "disable_os", Sequence: 1, Host: "gpu-01", StartedAt: now, Status: domain.StepStatusSucceeded},
		{ExecutionID: "step-exec", StepName: "drain_slurm", Sequence: 2, Host: "gpu-01", StartedAt: now.Add(time.Second), Status: domain.StepStatusRunning},
	}
	for _, step := range steps {
		s.CreateStep(context.Background(), step)
	}

	w := doAuthGet(srv, "/v1/switches/step-exec/steps")
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	var result []StepResponse
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result))
	}
	if result[0].StepName != "disable_os" || result[1].StepName != "drain_slurm" {
		t.Errorf("unexpected step order: %s, %s", result[0].StepName, result[1].StepName)
	}
	if result[0].Sequence != 1 || result[1].Sequence != 2 {
		t.Error("unexpected sequence values")
	}
}

func TestSteps_NotFound(t *testing.T) {
	srv, _ := setupHistoryServer(t)
	w := doAuthGet(srv, "/v1/switches/nonexistent/steps")
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetExecution_DetailFields(t *testing.T) {
	srv, s := setupHistoryServer(t)
	lockTime := time.Date(2026, 6, 1, 10, 5, 0, 0, time.UTC)
	exec := &domain.Execution{
		ID: "detail-1", NodeName: "gpu-01", Direction: domain.DirectionOpenStackToSlurm,
		OverallStatus: domain.OverallStatusActive, CurrentState: domain.StateLocked,
		RequestedAt: time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC), RequestedBy: "test",
		DesiredOwner: domain.OwnerSlurm, PreviousOwner: domain.OwnerOpenStack,
		StateVersion: 3, LockAcquiredAt: &lockTime,
		RequestedSlurmConstraint: "gpu-a100", RequestedSlurmPartition: "gpu-maint",
		PlaceholderJobID: "job-42",
	}
	s.CreateExecution(context.Background(), exec)

	w := doAuthGet(srv, "/v1/switches/detail-1")
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	var detail ExecutionDetail
	json.Unmarshal(w.Body.Bytes(), &detail)

	if detail.StateVersion != 3 {
		t.Errorf("state_version: got %d, want 3", detail.StateVersion)
	}
	if detail.DesiredOwner != "slurm" {
		t.Errorf("desired_owner: got %s, want slurm", detail.DesiredOwner)
	}
	if detail.RequestedSlurmConstraint != "gpu-a100" {
		t.Errorf("constraint: got %s", detail.RequestedSlurmConstraint)
	}
	if detail.PlaceholderJobID != "job-42" {
		t.Errorf("placeholder_job_id: got %s", detail.PlaceholderJobID)
	}
	if detail.LockAcquiredAt == nil {
		t.Error("lock_acquired_at should be set")
	}
}
