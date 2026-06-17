package slurm

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

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

func TestSubmitPlaceholderJob_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/slurm/v0.0.40/job/submit" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertSlurmHeaders(t, r, "cloud-user", "test-token")

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		job := body["job"].(map[string]any)
		if job["name"] != "gpu-switch-exec-1" {
			t.Errorf("unexpected job name: %v", job["name"])
		}

		json.NewEncoder(w).Encode(jobSubmitResponse{JobID: 42})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "test-token")
	result, err := client.SubmitPlaceholderJob(context.Background(), PlaceholderJobRequest{
		ExecutionID: "exec-1",
		Constraint:  "gpu-a100",
		Partition:   "gpu",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.JobID != "42" {
		t.Errorf("expected job_id=42, got %s", result.JobID)
	}
}

func TestSubmitPlaceholderJob_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(slurmErrorResponse{
			Errors: []slurmError{{Error: "invalid partition", Errno: 1}},
		})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "test-token")
	_, err := client.SubmitPlaceholderJob(context.Background(), PlaceholderJobRequest{
		ExecutionID: "exec-1",
		Constraint:  "gpu-a100",
		Partition:   "bad",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*SlurmAPIError)
	if !ok {
		t.Fatalf("expected SlurmAPIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", apiErr.StatusCode)
	}
	if len(apiErr.Messages) == 0 || apiErr.Messages[0] != "invalid partition" {
		t.Errorf("unexpected messages: %v", apiErr.Messages)
	}
}

func TestGetNodeState_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/slurm/v0.0.40/node/gpu-node-01" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertSlurmHeaders(t, r, "cloud-user", "test-token")

		json.NewEncoder(w).Encode(nodeInfoResponse{
			Nodes: []nodeInfo{{
				Name:        "gpu-node-01",
				State:       []string{"idle"},
				Gres:        "gpu:a100:4",
				AllocJobIDs: []int{100, 101},
			}},
		})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "test-token")
	state, err := client.GetNodeState(context.Background(), "gpu-node-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.NodeName != "gpu-node-01" {
		t.Errorf("unexpected node name: %s", state.NodeName)
	}
	if state.State != "idle" {
		t.Errorf("unexpected state: %s", state.State)
	}
	if len(state.GRES) != 1 || state.GRES[0] != "gpu:a100:4" {
		t.Errorf("unexpected GRES: %v", state.GRES)
	}
	if len(state.RunningJob) != 2 || state.RunningJob[0] != "100" {
		t.Errorf("unexpected running jobs: %v", state.RunningJob)
	}
}

func TestGetNodeState_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(slurmErrorResponse{
			Errors: []slurmError{{Error: "node not found", Errno: 2}},
		})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "test-token")
	_, err := client.GetNodeState(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*SlurmAPIError)
	if !ok {
		t.Fatalf("expected SlurmAPIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}
}

func TestDrainNode_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/slurm/v0.0.40/node/gpu-node-01" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertSlurmHeaders(t, r, "cloud-user", "test-token")

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		states, ok := body["state"].([]any)
		if !ok || len(states) != 1 || states[0] != "DRAIN" {
			t.Errorf("expected state=[DRAIN], got %v", body["state"])
		}
		if body["reason"] != "gpu switch in progress" {
			t.Errorf("unexpected reason: %v", body["reason"])
		}

		json.NewEncoder(w).Encode(slurmErrorResponse{})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "test-token")
	err := client.DrainNode(context.Background(), "gpu-node-01", "gpu switch in progress")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDrainNode_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(slurmErrorResponse{
			Errors: []slurmError{{Error: "internal failure", Errno: 500}},
		})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "test-token")
	err := client.DrainNode(context.Background(), "gpu-node-01", "test")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*SlurmAPIError)
	if !ok {
		t.Fatalf("expected SlurmAPIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", apiErr.StatusCode)
	}
}

func TestResumeNode_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/slurm/v0.0.40/node/gpu-node-01" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertSlurmHeaders(t, r, "cloud-user", "test-token")

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		states, ok := body["state"].([]any)
		if !ok || len(states) != 1 || states[0] != "RESUME" {
			t.Errorf("expected state=[RESUME], got %v", body["state"])
		}

		json.NewEncoder(w).Encode(slurmErrorResponse{})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "test-token")
	err := client.ResumeNode(context.Background(), "gpu-node-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelJob_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/slurm/v0.0.40/job/42" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertSlurmHeaders(t, r, "cloud-user", "test-token")
		json.NewEncoder(w).Encode(slurmErrorResponse{})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "test-token")
	err := client.CancelJob(context.Background(), "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelJob_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(slurmErrorResponse{
			Errors: []slurmError{{Error: "job not found", Errno: 2}},
		})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "test-token")
	err := client.CancelJob(context.Background(), "99999")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*SlurmAPIError)
	if !ok {
		t.Fatalf("expected SlurmAPIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}
}

func TestSlurmIdentityHeaders(t *testing.T) {
	var receivedUser, receivedToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUser = r.Header.Get("X-SLURM-USER-NAME")
		receivedToken = r.Header.Get("X-SLURM-USER-TOKEN")
		json.NewEncoder(w).Encode(slurmErrorResponse{})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "my-secret-jwt")
	_ = client.CancelJob(context.Background(), "1")

	if receivedUser != "cloud-user" {
		t.Errorf("expected X-SLURM-USER-NAME=cloud-user, got %q", receivedUser)
	}
	if receivedToken != "my-secret-jwt" {
		t.Errorf("expected X-SLURM-USER-TOKEN=my-secret-jwt, got %q", receivedToken)
	}
}

func TestCustomSlurmUser(t *testing.T) {
	var receivedUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUser = r.Header.Get("X-SLURM-USER-NAME")
		json.NewEncoder(w).Encode(slurmErrorResponse{})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "token", WithSlurmUser("custom-user"))
	_ = client.CancelJob(context.Background(), "1")

	if receivedUser != "custom-user" {
		t.Errorf("expected X-SLURM-USER-NAME=custom-user, got %q", receivedUser)
	}
}

func TestAdminCredentialsUsedForDrain(t *testing.T) {
	var receivedUser, receivedToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUser = r.Header.Get("X-SLURM-USER-NAME")
		receivedToken = r.Header.Get("X-SLURM-USER-TOKEN")
		json.NewEncoder(w).Encode(slurmErrorResponse{})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "workload-token",
		WithSlurmUser("cloud-user"),
		WithAdminCredentials("root", "admin-token"),
	)
	_ = client.DrainNode(context.Background(), "node-01", "test")

	if receivedUser != "root" {
		t.Errorf("expected admin user root, got %q", receivedUser)
	}
	if receivedToken != "admin-token" {
		t.Errorf("expected admin-token, got %q", receivedToken)
	}
}

func TestDrainNode_Idempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(slurmErrorResponse{
			Errors: []slurmError{{Error: "Node already drained", Errno: 42}},
		})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "test-token")
	err := client.DrainNode(context.Background(), "gpu-node-01", "test")
	if err != nil {
		t.Fatalf("expected nil for idempotent drain, got %v", err)
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		json.NewEncoder(w).Encode(slurmErrorResponse{})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "token", WithTimeout(50*time.Millisecond))
	err := client.CancelJob(context.Background(), "1")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if _, ok := err.(*SlurmAPIError); ok {
		t.Error("timeout should not be a SlurmAPIError")
	}
}

func TestConnectionRefused(t *testing.T) {
	client := NewRestClient("http://127.0.0.1:1", "token")
	err := client.CancelJob(context.Background(), "1")
	if err == nil {
		t.Fatal("expected connection error")
	}
	if _, ok := err.(*SlurmAPIError); ok {
		t.Error("connection error should not be a SlurmAPIError")
	}
}

func TestRequestLogSuccessfulWorkloadRequest(t *testing.T) {
	logger, logs := newCaptureLogger()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertSlurmHeaders(t, r, "cloud-user", "workload-token")
		json.NewEncoder(w).Encode(nodeInfoResponse{Nodes: []nodeInfo{{Name: "gpu-01", State: []string{"idle"}}}})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "workload-token", WithLogger(logger))
	_, err := client.GetNodeState(context.Background(), "gpu-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertSlurmRequestLog(t, logs.find("slurmrestd.request"), http.MethodGet, "/slurm/v0.0.40/node/gpu-01", "admin", "200", "")
	assertNoSensitiveValues(t, logs.find("slurmrestd.request"), "workload-token")
}

func TestRequestLogSuccessfulAdminRequest(t *testing.T) {
	logger, logs := newCaptureLogger()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertSlurmHeaders(t, r, "root", "admin-token")
		json.NewEncoder(w).Encode(slurmErrorResponse{})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "workload-token",
		WithLogger(logger),
		WithSlurmUser("cloud-user"),
		WithAdminCredentials("root", "admin-token"),
	)
	if err := client.DrainNode(context.Background(), "gpu-01", "maintenance"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertSlurmRequestLog(t, logs.find("slurmrestd.request"), http.MethodPost, "/slurm/v0.0.40/node/gpu-01", "admin", "200", "")
	assertNoSensitiveValues(t, logs.find("slurmrestd.request"), "admin-token", "workload-token")
}

func TestRequestLogAPIRejection(t *testing.T) {
	logger, logs := newCaptureLogger()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(slurmErrorResponse{Errors: []slurmError{{Error: "invalid partition", Errno: 1}}})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "secret-token", WithLogger(logger))
	_, err := client.SubmitPlaceholderJob(context.Background(), PlaceholderJobRequest{ExecutionID: "exec-1", Partition: "bad"})
	if err == nil {
		t.Fatal("expected error")
	}

	assertSlurmRequestLog(t, logs.find("slurmrestd.request"), http.MethodPost, "/slurm/v0.0.40/job/submit", "workload", "400", "invalid partition")
	assertNoSensitiveValues(t, logs.find("slurmrestd.request"), "secret-token")
}

func TestRequestLogTransportError(t *testing.T) {
	logger, logs := newCaptureLogger()
	client := NewRestClient("http://127.0.0.1:1", "secret-token", WithLogger(logger))
	err := client.CancelJob(context.Background(), "1")
	if err == nil {
		t.Fatal("expected error")
	}

	rec := logs.find("slurmrestd.request")
	if rec == nil {
		t.Fatal("expected slurmrestd.request log")
	}
	assertSlurmRequestLog(t, rec, http.MethodDelete, "/slurm/v0.0.40/job/1", "admin", "", "")
	if rec.Attrs["error"] == "" {
		t.Fatal("expected transport error attr")
	}
	assertNoSensitiveValues(t, rec, "secret-token")
}

// fakeAdminTokenProvider scripts token issuance for admin-request tests.
type fakeAdminTokenProvider struct {
	tokens     []string
	tokenCalls int
	renewCalls int
}

func (p *fakeAdminTokenProvider) next() string {
	idx := p.tokenCalls + p.renewCalls - 1
	if idx >= len(p.tokens) {
		idx = len(p.tokens) - 1
	}
	return p.tokens[idx]
}

func (p *fakeAdminTokenProvider) Token(_ context.Context) (string, error) {
	p.tokenCalls++
	return p.next(), nil
}

func (p *fakeAdminTokenProvider) Renew(_ context.Context, _ string) (string, error) {
	p.renewCalls++
	return p.next(), nil
}

func TestAdminRequestRetriesOnceAfterAuthFailure(t *testing.T) {
	var sentTokens []string
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentTokens = append(sentTokens, r.Header.Get("X-SLURM-USER-TOKEN"))
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(slurmErrorResponse{Errors: []slurmError{{Error: "Invalid token", Errno: 9001}}})
			return
		}
		json.NewEncoder(w).Encode(slurmErrorResponse{})
	}))
	defer srv.Close()

	provider := &fakeAdminTokenProvider{tokens: []string{"stale-token", "fresh-token"}}
	client := NewRestClient(srv.URL, "workload-token", WithAdminTokenProvider("root", provider))

	if err := client.DrainNode(context.Background(), "gpu-01", "switch"); err != nil {
		t.Fatalf("DrainNode() error = %v", err)
	}

	if calls != 2 {
		t.Fatalf("server calls = %d, want 2 (one retry)", calls)
	}
	if provider.renewCalls != 1 {
		t.Fatalf("renew calls = %d, want 1", provider.renewCalls)
	}
	if len(sentTokens) != 2 || sentTokens[0] != "stale-token" || sentTokens[1] != "fresh-token" {
		t.Fatalf("sent tokens = %v, want [stale-token fresh-token]", sentTokens)
	}
}

func TestAdminRequestReturnsErrorAfterRepeatedAuthFailure(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(slurmErrorResponse{Errors: []slurmError{{Error: "Invalid token", Errno: 9001}}})
	}))
	defer srv.Close()

	provider := &fakeAdminTokenProvider{tokens: []string{"t1", "t2"}}
	client := NewRestClient(srv.URL, "workload-token", WithAdminTokenProvider("root", provider))

	err := client.CancelJob(context.Background(), "42")
	if err == nil {
		t.Fatal("expected error after repeated auth failure")
	}
	apiErr, ok := err.(*SlurmAPIError)
	if !ok {
		t.Fatalf("expected SlurmAPIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", apiErr.StatusCode)
	}
	if calls != 2 {
		t.Fatalf("server calls = %d, want 2 (single retry, no further retries)", calls)
	}
	if provider.renewCalls != 1 {
		t.Fatalf("renew calls = %d, want 1", provider.renewCalls)
	}
}

func TestAdminRequestDoesNotRetryNonAuthFailure(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(slurmErrorResponse{Errors: []slurmError{{Error: "invalid node state", Errno: 2}}})
	}))
	defer srv.Close()

	provider := &fakeAdminTokenProvider{tokens: []string{"t1", "t2"}}
	client := NewRestClient(srv.URL, "workload-token", WithAdminTokenProvider("root", provider))

	err := client.ResumeNode(context.Background(), "gpu-01")
	if err == nil {
		t.Fatal("expected error for non-auth failure")
	}
	if calls != 1 {
		t.Fatalf("server calls = %d, want 1 (no retry)", calls)
	}
	if provider.renewCalls != 0 {
		t.Fatalf("renew calls = %d, want 0 for non-auth failure", provider.renewCalls)
	}
}

func TestAdminRequestSucceedsWithCachedToken(t *testing.T) {
	var sentToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentToken = r.Header.Get("X-SLURM-USER-TOKEN")
		json.NewEncoder(w).Encode(slurmErrorResponse{})
	}))
	defer srv.Close()

	provider := &fakeAdminTokenProvider{tokens: []string{"cached-token"}}
	client := NewRestClient(srv.URL, "workload-token", WithAdminTokenProvider("root", provider))

	if err := client.DrainNode(context.Background(), "gpu-01", "switch"); err != nil {
		t.Fatalf("DrainNode() error = %v", err)
	}
	if sentToken != "cached-token" {
		t.Fatalf("sent token = %q, want cached-token", sentToken)
	}
	if provider.renewCalls != 0 {
		t.Fatalf("renew calls = %d, want 0 on success", provider.renewCalls)
	}
}

func assertSlurmHeaders(t *testing.T, r *http.Request, expectedUser, expectedToken string) {
	t.Helper()
	user := r.Header.Get("X-SLURM-USER-NAME")
	token := r.Header.Get("X-SLURM-USER-TOKEN")
	if user != expectedUser {
		t.Errorf("expected X-SLURM-USER-NAME=%q, got %q", expectedUser, user)
	}
	if token != expectedToken {
		t.Errorf("expected X-SLURM-USER-TOKEN=%q, got %q", expectedToken, token)
	}
}

func assertSlurmRequestLog(t *testing.T, rec *capturedRecord, method, path, identity, statusCode, apiError string) {
	t.Helper()
	if rec == nil {
		t.Fatal("expected slurmrestd.request log")
	}
	if rec.Attrs["component"] != "slurmrestd_client" {
		t.Fatalf("component = %q, want %q", rec.Attrs["component"], "slurmrestd_client")
	}
	if rec.Attrs["event"] != "slurmrestd.request" {
		t.Fatalf("event = %q, want %q", rec.Attrs["event"], "slurmrestd.request")
	}
	if rec.Attrs["method"] != method {
		t.Fatalf("method = %q, want %q", rec.Attrs["method"], method)
	}
	if rec.Attrs["path"] != path {
		t.Fatalf("path = %q, want %q", rec.Attrs["path"], path)
	}
	if rec.Attrs["identity"] != identity {
		t.Fatalf("identity = %q, want %q", rec.Attrs["identity"], identity)
	}
	if statusCode == "" {
		if _, ok := rec.Attrs["status_code"]; ok {
			t.Fatalf("status_code = %q, want absent", rec.Attrs["status_code"])
		}
	} else if rec.Attrs["status_code"] != statusCode {
		t.Fatalf("status_code = %q, want %q", rec.Attrs["status_code"], statusCode)
	}
	if apiError == "" {
		if _, ok := rec.Attrs["api_error"]; ok {
			t.Fatalf("api_error = %q, want absent", rec.Attrs["api_error"])
		}
	} else if rec.Attrs["api_error"] != apiError {
		t.Fatalf("api_error = %q, want %q", rec.Attrs["api_error"], apiError)
	}
	if rec.Attrs["latency"] == "" {
		t.Fatal("expected latency attr")
	}
}

func assertNoSensitiveValues(t *testing.T, rec *capturedRecord, forbiddenValues ...string) {
	t.Helper()
	if rec == nil {
		t.Fatal("expected slurmrestd.request log")
	}
	if _, ok := rec.Attrs["body"]; ok {
		t.Fatal("log should not include body attr")
	}
	if _, ok := rec.Attrs["token"]; ok {
		t.Fatal("log should not include token attr")
	}
	for key, value := range rec.Attrs {
		for _, forbidden := range forbiddenValues {
			if forbidden != "" && strings.Contains(value, forbidden) {
				t.Fatalf("log attr %q leaked sensitive value %q", key, forbidden)
			}
		}
	}
}

func TestSubmitPlaceholderJob_AccountAndHomePaths(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertSlurmHeaders(t, r, "alice", "jwt-override")

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		job := body["job"].(map[string]any)

		if job["account"] != "proj-123" {
			t.Errorf("expected account=proj-123, got %v", job["account"])
		}
		if job["current_working_directory"] != "/home/alice" {
			t.Errorf("expected cwd=/home/alice, got %v", job["current_working_directory"])
		}
		stdout := job["standard_output"].(string)
		if !strings.HasPrefix(stdout, "/home/alice/") {
			t.Errorf("expected stdout under /home/alice/, got %s", stdout)
		}
		stderr := job["standard_error"].(string)
		if !strings.HasPrefix(stderr, "/home/alice/") {
			t.Errorf("expected stderr under /home/alice/, got %s", stderr)
		}

		script := body["script"].(string)
		if !strings.Contains(script, "export SLURM_API_USER=alice") {
			t.Error("script should export SLURM_API_USER=alice")
		}
		if !strings.Contains(script, "export SLURM_JWT_TOKEN=jwt-override") {
			t.Error("script should export SLURM_JWT_TOKEN=jwt-override")
		}

		json.NewEncoder(w).Encode(jobSubmitResponse{JobID: 99})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "daemon-token", WithSlurmUser("cloud-user"))
	result, err := client.SubmitPlaceholderJob(context.Background(), PlaceholderJobRequest{
		ExecutionID:   "exec-2",
		Constraint:    "gpu-a100",
		Partition:     "gpu-maint",
		Account:       "proj-123",
		WorkloadUser:  "alice",
		WorkloadToken: "jwt-override",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.JobID != "99" {
		t.Errorf("expected job_id=99, got %s", result.JobID)
	}
}

func TestSubmitPlaceholderJob_NoAccountOmitsField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		job := body["job"].(map[string]any)

		if _, exists := job["account"]; exists {
			t.Error("account field should not be present when not requested")
		}
		if job["current_working_directory"] != "/home/cloud-user" {
			t.Errorf("expected cwd=/home/cloud-user, got %v", job["current_working_directory"])
		}

		json.NewEncoder(w).Encode(jobSubmitResponse{JobID: 100})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "daemon-token")
	_, err := client.SubmitPlaceholderJob(context.Background(), PlaceholderJobRequest{
		ExecutionID: "exec-3",
		Constraint:  "gpu-a100",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetJobState_NonTerminal(t *testing.T) {
	for _, state := range []string{"PENDING", "RUNNING", "CONFIGURING"} {
		state := state
		t.Run(state, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/slurm/v0.0.40/job/12345" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				assertSlurmHeaders(t, r, "alice", "workload-token")
				json.NewEncoder(w).Encode(jobStateResponse{
					Jobs: []jobInfo{{JobID: 12345, JobState: []string{state}}},
				})
			}))
			defer srv.Close()

			client := NewRestClient(srv.URL, "daemon-token")
			result, err := client.GetJobState(context.Background(), "12345", WorkloadIdentity{User: "alice", Token: "workload-token"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.State != state {
				t.Errorf("State = %q, want %q", result.State, state)
			}
			if result.IsTerminal {
				t.Errorf("IsTerminal = true, want false for state %q", state)
			}
		})
	}
}

func TestGetJobState_TerminalFailure(t *testing.T) {
	for _, state := range []string{"FAILED", "BOOT_FAIL", "CANCELLED", "TIMEOUT", "NODE_FAIL"} {
		state := state
		t.Run(state, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assertSlurmHeaders(t, r, "alice", "workload-token")
				json.NewEncoder(w).Encode(jobStateResponse{
					Jobs: []jobInfo{{JobID: 12345, JobState: []string{state}}},
				})
			}))
			defer srv.Close()

			client := NewRestClient(srv.URL, "daemon-token")
			result, err := client.GetJobState(context.Background(), "12345", WorkloadIdentity{User: "alice", Token: "workload-token"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.State != state {
				t.Errorf("State = %q, want %q", result.State, state)
			}
			if !result.IsTerminal {
				t.Errorf("IsTerminal = false, want true for state %q", state)
			}
		})
	}
}

func TestGetJobState_TerminalCompleted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertSlurmHeaders(t, r, "alice", "workload-token")
		json.NewEncoder(w).Encode(jobStateResponse{
			Jobs: []jobInfo{{JobID: 12345, JobState: []string{"COMPLETED"}}},
		})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "daemon-token")
	result, err := client.GetJobState(context.Background(), "12345", WorkloadIdentity{User: "alice", Token: "workload-token"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.State != "COMPLETED" {
		t.Errorf("State = %q, want COMPLETED", result.State)
	}
	if !result.IsTerminal {
		t.Error("IsTerminal = false, want true for COMPLETED")
	}
}

func TestGetJobState_SlurmAPIRejection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(slurmErrorResponse{
			Errors: []slurmError{{Error: "job not found", Errno: 2}},
		})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "daemon-token")
	_, err := client.GetJobState(context.Background(), "99999", WorkloadIdentity{User: "alice", Token: "workload-token"})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*SlurmAPIError)
	if !ok {
		t.Fatalf("expected SlurmAPIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", apiErr.StatusCode)
	}
}

func TestGetJobState_UsesWorkloadIdentityHeaders(t *testing.T) {
	var receivedUser, receivedToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUser = r.Header.Get("X-SLURM-USER-NAME")
		receivedToken = r.Header.Get("X-SLURM-USER-TOKEN")
		json.NewEncoder(w).Encode(jobStateResponse{
			Jobs: []jobInfo{{JobID: 1, JobState: []string{"RUNNING"}}},
		})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "daemon-token",
		WithAdminCredentials("root", "admin-token"),
	)
	_, err := client.GetJobState(context.Background(), "1", WorkloadIdentity{User: "alice", Token: "alice-jwt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedUser != "alice" {
		t.Errorf("X-SLURM-USER-NAME = %q, want alice", receivedUser)
	}
	if receivedToken != "alice-jwt" {
		t.Errorf("X-SLURM-USER-TOKEN = %q, want alice-jwt", receivedToken)
	}
}
