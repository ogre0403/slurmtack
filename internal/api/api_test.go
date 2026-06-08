package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/service"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

type apiTestSlurmNodeStateReader struct {
	nodeState *slurm.NodeState
	err       error
}

type capturedRecord struct {
	Message string
	Attrs   map[string]string
}

type captureStore struct {
	mu      sync.Mutex
	records []*capturedRecord
}

func (s *captureStore) find(msg string) *capturedRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.records {
		if r.Message == msg {
			return r
		}
	}
	return nil
}

func (s *captureStore) findLast(msg string) *capturedRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.records) - 1; i >= 0; i-- {
		if s.records[i].Message == msg {
			return s.records[i]
		}
	}
	return nil
}

type captureHandler struct {
	shared *captureStore
	attrs  []slog.Attr
}

func newCaptureLogger() (*slog.Logger, *captureStore) {
	cs := &captureStore{}
	return slog.New(&captureHandler{shared: cs}), cs
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	rec := &capturedRecord{Message: r.Message, Attrs: make(map[string]string)}
	for _, attr := range h.attrs {
		rec.Attrs[attr.Key] = attr.Value.String()
	}
	r.Attrs(func(attr slog.Attr) bool {
		rec.Attrs[attr.Key] = attr.Value.String()
		return true
	})
	h.shared.mu.Lock()
	h.shared.records = append(h.shared.records, rec)
	h.shared.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(merged, h.attrs)
	copy(merged[len(h.attrs):], attrs)
	return &captureHandler{shared: h.shared, attrs: merged}
}

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func (r *apiTestSlurmNodeStateReader) GetNodeState(_ context.Context, _ string) (*slurm.NodeState, error) {
	return r.nodeState, r.err
}

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	srv, _ := setupTestServerWithStore(t)
	return srv
}

func setupTestServerWithStore(t *testing.T, readers ...service.SlurmNodeStateReader) (*Server, *store.SQLiteStore) {
	t.Helper()
	logger, _ := newCaptureLogger()
	return setupTestServerWithStoreAndLogger(t, logger, readers...)
}

var testJWTManager = NewJWTManager([]byte("test-secret-key-32-bytes-long!!"), time.Hour)

func testAuthToken(t *testing.T) string {
	t.Helper()
	token, err := testJWTManager.Generate("test-operator")
	if err != nil {
		t.Fatalf("generate test JWT: %v", err)
	}
	return token
}

func setupTestServerWithStoreAndLogger(t *testing.T, logger *slog.Logger, readers ...service.SlurmNodeStateReader) (*Server, *store.SQLiteStore) {
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

	svc := service.NewSwitchService(sqlStore, nil).
		WithSlurmWorkloadDefaults("cloud-user", "test-token").
		WithPlaceholderSIFDefaults("slurmtack/build/output", "placeholder-agent.sif")
	if len(readers) > 0 {
		svc = svc.WithSlurmNodeStateReader(readers[0])
	}
	return NewServer(":0", sqlStore, svc, nil, logger, WithJWTAuth(testJWTManager, nil)), sqlStore
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
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
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

func TestCreateSwitchWithSlurmPartition(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"direction":"slurm_to_openstack","requested_by":"operator-1","slurm_constraint":"gpu-a100","slurm_partition":"gpu-maint"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
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
}

func TestCreateOpenStackToSlurmStartsAwaitingTargetNode(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"direction":"openstack_to_slurm","requested_by":"operator-1","node_name":"gpu-01"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	var createResp SwitchResponse
	json.Unmarshal(w.Body.Bytes(), &createResp)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/switches/"+createResp.ExecutionID, nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status ExecutionStatus
	json.Unmarshal(w.Body.Bytes(), &status)
	if status.CurrentState != "awaiting_target_node" {
		t.Fatalf("CurrentState = %q, want awaiting_target_node", status.CurrentState)
	}
	if status.NodeName != "gpu-01" {
		t.Fatalf("NodeName = %q, want gpu-01", status.NodeName)
	}
}

func TestCreateOpenStackToSlurmRejectsMissingNodeName(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"direction":"openstack_to_slurm","requested_by":"operator-1"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateOpenStackToSlurmRejectsDuplicateSlurmOwnership(t *testing.T) {
	srv, sqlStore := setupTestServerWithStore(t, &apiTestSlurmNodeStateReader{nodeState: &slurm.NodeState{NodeName: "gpu-01", State: "idle"}})

	body := `{"direction":"openstack_to_slurm","requested_by":"operator-1","node_name":"gpu-01"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error == "" || !strings.Contains(resp.Error, "already under Slurm ownership") {
		t.Fatalf("unexpected error response: %+v", resp)
	}

	executions, err := sqlStore.ListExecutions(context.Background(), "")
	if err != nil {
		t.Fatalf("ListExecutions() error = %v", err)
	}
	if len(executions) != 0 {
		t.Fatalf("execution count = %d, want 0", len(executions))
	}
}

func TestCreateOpenStackToSlurmLookupFailureReturnsServerError(t *testing.T) {
	srv := setupTestServerWithReader(t, &apiTestSlurmNodeStateReader{err: errors.New("slurm unavailable")})

	body := `{"direction":"openstack_to_slurm","requested_by":"operator-1","node_name":"gpu-01"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func setupTestServerWithReader(t *testing.T, reader service.SlurmNodeStateReader) *Server {
	t.Helper()
	srv, _ := setupTestServerWithStore(t, reader)
	return srv
}

func TestCreateSwitchInvalidDirection(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"direction":"invalid","requested_by":"operator-1"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
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
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetSwitch(t *testing.T) {
	srv := setupTestServer(t)

	// Create one first
	body := `{"direction":"slurm_to_openstack","requested_by":"op","slurm_constraint":"gpu-a100"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	var createResp SwitchResponse
	json.Unmarshal(w.Body.Bytes(), &createResp)

	// Get it
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/switches/"+createResp.ExecutionID, nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var status ExecutionStatus
	json.Unmarshal(w.Body.Bytes(), &status)
	if status.NodeName != "" || status.Direction != "slurm_to_openstack" {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestCreateSlurmToOpenStackRejectsNodeName(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"direction":"slurm_to_openstack","requested_by":"operator-1","node_name":"gpu-01"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Error, "node_name is not accepted for slurm_to_openstack") {
		t.Fatalf("unexpected error response: %+v", resp)
	}
}

func TestGetSwitchNotFound(t *testing.T) {
	srv := setupTestServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/switches/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetSwitchExposesRequestedSlurmAccount(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"direction":"slurm_to_openstack","requested_by":"op","slurm_account":"proj-123","slurm_user":"alice","slurm_user_token":"jwt-abc"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var createResp SwitchResponse
	json.Unmarshal(w.Body.Bytes(), &createResp)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/switches/"+createResp.ExecutionID, nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var detail map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &detail)
	if detail["requested_slurm_account"] != "proj-123" {
		t.Fatalf("requested_slurm_account = %v, want proj-123", detail["requested_slurm_account"])
	}
	if _, exists := detail["slurm_workload_user"]; exists {
		t.Fatal("slurm_workload_user should not be exposed in response")
	}
	if _, exists := detail["slurm_workload_token"]; exists {
		t.Fatal("slurm_workload_token should not be exposed in response")
	}
}

func TestCreateSlurmToOpenStackRejectsIncompleteCredentials(t *testing.T) {
	srv := setupTestServer(t)

	body := `{"direction":"slurm_to_openstack","requested_by":"op","slurm_user":"alice"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	var resp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Error, "slurm_user and slurm_user_token must be provided together") {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
}

func TestListSwitches(t *testing.T) {
	srv := setupTestServer(t)

	// Create two node-bound executions using openstack_to_slurm (which requires node_name)
	for _, node := range []string{"gpu-01", "gpu-02"} {
		body := `{"direction":"openstack_to_slurm","requested_by":"op","node_name":"` + node + `"}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/v1/switches", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
		req.Header.Set("Content-Type", "application/json")
		srv.Engine().ServeHTTP(w, req)
	}

	// List all
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/switches", nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
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
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	srv.Engine().ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("expected 1 filtered, got %d", len(list))
	}

	// Filter by status
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v1/switches?status=active", nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	srv.Engine().ServeHTTP(w, req)

	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatalf("expected 2 active, got %d", len(list))
	}
}

func TestCancelUnknownID(t *testing.T) {
	srv := setupTestServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/switches/some-id/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown execution, got %d", w.Code)
	}
}

func TestAccessLogSuccessfulV1Request(t *testing.T) {
	logger, logs := newCaptureLogger()
	srv := setupTestServerWithLogger(t, logger)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/switches", bytes.NewBufferString(`{"direction":"slurm_to_openstack","requested_by":"operator-1","slurm_constraint":"gpu-a100"}`))
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.50:1234"
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}

	var createResp SwitchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/v1/switches/"+createResp.ExecutionID, nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	req.RemoteAddr = "192.0.2.50:1234"
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	assertAPILogRecord(t, logs.findLast("api.request"), http.MethodGet, "/v1/switches/:id", "/v1/switches/"+createResp.ExecutionID, http.StatusOK, "192.0.2.50")
	assertNoAPISensitiveAttrs(t, logs.findLast("api.request"), testAuthToken(t))
}

func TestAccessLogAuthorizationFailure(t *testing.T) {
	logger, logs := newCaptureLogger()
	srv := setupTestServerWithLogger(t, logger)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/v1/switches", bytes.NewBufferString(`{"requested_by":"operator-1"}`))
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.7:9999"
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	assertAPILogRecord(t, logs.findLast("api.request"), http.MethodPost, "/v1/switches", "/v1/switches", http.StatusUnauthorized, "198.51.100.7")
	assertNoAPISensitiveAttrs(t, logs.findLast("api.request"), "secret-token", "operator-1")
}

func TestAccessLogHealthRequest(t *testing.T) {
	logger, logs := newCaptureLogger()
	srv := setupTestServerWithLogger(t, logger)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "203.0.113.9:8080"
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	assertAPILogRecord(t, logs.findLast("api.request"), http.MethodGet, "/health", "/health", http.StatusOK, "203.0.113.9")
}

func setupTestServerWithLogger(t *testing.T, logger *slog.Logger) *Server {
	t.Helper()
	srv, _ := setupTestServerWithStoreAndLogger(t, logger)
	return srv
}

func assertAPILog(t *testing.T, logs *captureStore, method, route, path string, statusCode int, clientIP string) {
	t.Helper()
	assertAPILogRecord(t, logs.find("api.request"), method, route, path, statusCode, clientIP)
}

func assertAPILogRecord(t *testing.T, rec *capturedRecord, method, route, path string, statusCode int, clientIP string) {
	t.Helper()
	if rec == nil {
		t.Fatal("expected api.request log")
	}
	if rec.Attrs["component"] != "api" {
		t.Fatalf("component = %q, want %q", rec.Attrs["component"], "api")
	}
	if rec.Attrs["event"] != "api.request" {
		t.Fatalf("event = %q, want %q", rec.Attrs["event"], "api.request")
	}
	if rec.Attrs["method"] != method {
		t.Fatalf("method = %q, want %q", rec.Attrs["method"], method)
	}
	if rec.Attrs["route"] != route {
		t.Fatalf("route = %q, want %q", rec.Attrs["route"], route)
	}
	if rec.Attrs["path"] != path {
		t.Fatalf("path = %q, want %q", rec.Attrs["path"], path)
	}
	if rec.Attrs["status_code"] != strconv.Itoa(statusCode) {
		t.Fatalf("status_code = %q, want %d", rec.Attrs["status_code"], statusCode)
	}
	if rec.Attrs["client_ip"] != clientIP {
		t.Fatalf("client_ip = %q, want %q", rec.Attrs["client_ip"], clientIP)
	}
	if rec.Attrs["latency"] == "" {
		t.Fatal("expected latency attr")
	}
}

func assertNoAPISensitiveAttrs(t *testing.T, rec *capturedRecord, forbiddenValues ...string) {
	t.Helper()
	if rec == nil {
		t.Fatal("expected api.request log")
	}
	for key, value := range rec.Attrs {
		for _, forbidden := range forbiddenValues {
			if forbidden != "" && strings.Contains(value, forbidden) {
				t.Fatalf("log attr %q leaked sensitive value %q", key, forbidden)
			}
		}
	}
	if _, ok := rec.Attrs["authorization"]; ok {
		t.Fatal("log should not include authorization attr")
	}
	if _, ok := rec.Attrs["body"]; ok {
		t.Fatal("log should not include body attr")
	}
}
