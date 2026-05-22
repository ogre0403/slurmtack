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
		if r.Method != http.MethodPost || r.URL.Path != "/slurm/v0.0.38/job/submit" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertBearer(t, r, "test-token")

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		job := body["job"].(map[string]any)
		if job["name"] != "gpu-switch-exec-1" {
			t.Errorf("unexpected job name: %v", job["name"])
		}
		if job["exclusive"] != true {
			t.Errorf("expected exclusive=true")
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
		if r.Method != http.MethodGet || r.URL.Path != "/slurm/v0.0.38/node/gpu-node-01" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertBearer(t, r, "test-token")

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
		if r.Method != http.MethodPost || r.URL.Path != "/slurm/v0.0.38/node/gpu-node-01" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertBearer(t, r, "test-token")

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["state"] != "drain" {
			t.Errorf("expected state=drain, got %v", body["state"])
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
		if r.Method != http.MethodPost || r.URL.Path != "/slurm/v0.0.38/node/gpu-node-01" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertBearer(t, r, "test-token")

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["state"] != "resume" {
			t.Errorf("expected state=resume, got %v", body["state"])
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
		if r.Method != http.MethodDelete || r.URL.Path != "/slurm/v0.0.38/job/42" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		assertBearer(t, r, "test-token")
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

func TestJWTHeaderPresent(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(slurmErrorResponse{})
	}))
	defer srv.Close()

	client := NewRestClient(srv.URL, "my-secret-jwt")
	_ = client.CancelJob(context.Background(), "1")

	if receivedAuth != "Bearer my-secret-jwt" {
		t.Errorf("expected 'Bearer my-secret-jwt', got %q", receivedAuth)
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

func assertBearer(t *testing.T, r *http.Request, expected string) {
	t.Helper()
	auth := r.Header.Get("Authorization")
	if auth != "Bearer "+expected {
		t.Errorf("expected 'Bearer %s', got %q", expected, auth)
	}
}
