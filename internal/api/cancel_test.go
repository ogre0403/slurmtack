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

// setupCancelServer creates a server and inserts an execution in the given state.
func setupCancelServer(t *testing.T, state domain.SwitchState, dir domain.SwitchDirection) (*Server, string) {
	t.Helper()
	_, sqlStore := setupTestServerWithStore(t)

	exec := &domain.Execution{
		ID:            "cancel-test-exec",
		NodeName:      "gpu-01",
		Direction:     dir,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  state,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  0,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := sqlStore.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	svc := service.NewSwitchService(sqlStore, nil)
	srv := NewServer(":0", "test-token", sqlStore, svc, nil)
	return srv, exec.ID
}

func cancelRequest(t *testing.T, srv *Server, id string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches/"+id+"/cancel", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	srv.Engine().ServeHTTP(w, req)
	return w
}

func TestCancelEndpoint_AcceptsAwaitingTargetNode(t *testing.T) {
	srv, id := setupCancelServer(t, domain.StateAwaitingTargetNode, domain.DirectionOpenStackToSlurm)
	w := cancelRequest(t, srv, id)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var resp SwitchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.ExecutionID != id {
		t.Fatalf("execution_id = %q, want %q", resp.ExecutionID, id)
	}
	if resp.StatusURL != "/v1/switches/"+id {
		t.Fatalf("status_url = %q, want /v1/switches/%s", resp.StatusURL, id)
	}
}

func TestCancelEndpoint_AcceptsAwaitingSourceAllocation(t *testing.T) {
	srv, id := setupCancelServer(t, domain.StateAwaitingSourceAllocation, domain.DirectionSlurmToOpenStack)
	w := cancelRequest(t, srv, id)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCancelEndpoint_AcceptsSourceQuiescing(t *testing.T) {
	srv, id := setupCancelServer(t, domain.StateSourceQuiescing, domain.DirectionOpenStackToSlurm)
	w := cancelRequest(t, srv, id)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCancelEndpoint_RejectsRebooting(t *testing.T) {
	srv, id := setupCancelServer(t, domain.StateRebooting, domain.DirectionSlurmToOpenStack)
	w := cancelRequest(t, srv, id)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	var resp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error == "" {
		t.Fatal("expected non-empty error")
	}
}

func TestCancelEndpoint_NotFound(t *testing.T) {
	srv := setupTestServer(t)
	w := cancelRequest(t, srv, "nonexistent-id")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestCancelEndpoint_IdempotentOnCancelling(t *testing.T) {
	_, sqlStore := setupTestServerWithStore(t)

	exec := &domain.Execution{
		ID:                      "cancel-idempotent-exec",
		NodeName:                "gpu-01",
		Direction:               domain.DirectionSlurmToOpenStack,
		RequestedBy:             "test",
		RequestedAt:             time.Now(),
		CurrentState:            domain.StateCancelling,
		CancellationSourceState: domain.StateAwaitingSourceAllocation,
		DesiredOwner:            domain.OwnerOpenStack,
		PreviousOwner:           domain.OwnerSlurm,
		StateVersion:            1,
		OverallStatus:           domain.OverallStatusActive,
	}
	if err := sqlStore.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	svc := service.NewSwitchService(sqlStore, nil)
	srv := NewServer(":0", "test-token", sqlStore, svc, nil)

	w := cancelRequest(t, srv, exec.ID)
	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for idempotent cancel on cancelling, got %d: %s", w.Code, w.Body.String())
	}

	fresh, _ := sqlStore.GetExecution(context.Background(), exec.ID)
	if fresh.CurrentState != domain.StateCancelling {
		t.Fatalf("state = %s, want still cancelling", fresh.CurrentState)
	}
}

// TestCancelStub was replaced; this validates the updated behavior is consistent
// with the old test expectation that a random non-existent id returns 404 (not 501).
func TestCancelEndpoint_PreviousStubBehaviorReplaced(t *testing.T) {
	srv := setupTestServer(t)
	w := cancelRequest(t, srv, "some-nonexistent-id")
	// Old stub returned 501; new implementation returns 404 for unknown IDs.
	if w.Code == http.StatusNotImplemented {
		t.Fatal("cancel endpoint should no longer return 501")
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown execution, got %d", w.Code)
	}
}

// Ensure store.ErrNotFound is imported via the store package in the test.
var _ = store.ErrNotFound
