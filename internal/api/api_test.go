package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/slurmtack/slurmtack/internal/service"
	"github.com/slurmtack/slurmtack/internal/store"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	f, err := os.CreateTemp("", "slurmtack-api-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	sqlStore, err := store.NewSQLiteStore(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlStore.Close() })

	svc := service.NewSwitchService(sqlStore, nil)
	return NewServer(":0", "test-token", sqlStore, svc)
}

func TestHealthEndpoint(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %s", body["status"])
	}
}

func TestAuthRequired(t *testing.T) {
	srv := setupTestServer(t)

	// No auth header
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/switches", nil)
	srv.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	// Wrong token
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/switches", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	srv.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreateSwitch(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"direction":"slurm_to_openstack","requested_by":"operator-1","slurm_constraint":"gpu-a100"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp SwitchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.ExecutionID == "" {
		t.Fatal("expected non-empty execution_id")
	}
	if resp.StatusURL != "/v1/switches/"+resp.ExecutionID {
		t.Fatalf("unexpected status_url: %s", resp.StatusURL)
	}
}

func TestCreateSwitchInvalidDirection(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"direction":"invalid","requested_by":"operator-1"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateSwitchMissingField(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"requested_by":"operator-1"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetSwitch(t *testing.T) {
	srv := setupTestServer(t)

	// Create one first
	body := `{"direction":"slurm_to_openstack","requested_by":"op","node_name":"gpu-01"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	var createResp SwitchResponse
	json.Unmarshal(w.Body.Bytes(), &createResp)

	// Get it
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/switches/"+createResp.ExecutionID, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var status ExecutionStatus
	json.Unmarshal(w.Body.Bytes(), &status)
	if status.NodeName != "gpu-01" || status.Direction != "slurm_to_openstack" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestGetSwitchNotFound(t *testing.T) {
	srv := setupTestServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/switches/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListSwitches(t *testing.T) {
	srv := setupTestServer(t)

	// Create two
	for _, node := range []string{"gpu-01", "gpu-02"} {
		body := `{"direction":"slurm_to_openstack","requested_by":"op","node_name":"` + node + `"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		srv.Engine().ServeHTTP(w, req)
	}

	// List all
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/switches", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list []ExecutionStatus
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}

	// Filter by node
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/switches?node=gpu-01", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	srv.Engine().ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("expected 1 filtered, got %d", len(list))
	}

	// Filter by status
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/switches?status=active", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	srv.Engine().ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatalf("expected 2 active, got %d", len(list))
	}
}

func TestCancelStub(t *testing.T) {
	srv := setupTestServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches/some-id/cancel", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}
}
