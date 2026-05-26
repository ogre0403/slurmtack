package slurm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
