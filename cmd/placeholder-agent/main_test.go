package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoadConfig_MissingExecutionID(t *testing.T) {
	t.Setenv("EXECUTION_ID", "")
	t.Setenv("AMQP_URL", "amqp://localhost")
	t.Setenv("SLURM_API_URL", "http://localhost")
	t.Setenv("SLURM_JWT_TOKEN", "token")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for missing EXECUTION_ID")
	}
}

func TestLoadConfig_MissingAMQPURL(t *testing.T) {
	t.Setenv("EXECUTION_ID", "exec-1")
	t.Setenv("AMQP_URL", "")
	t.Setenv("SLURM_API_URL", "http://localhost")
	t.Setenv("SLURM_JWT_TOKEN", "token")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("expected error for missing AMQP_URL")
	}
}

func TestLoadConfig_AllPresent(t *testing.T) {
	t.Setenv("EXECUTION_ID", "exec-1")
	t.Setenv("AMQP_URL", "amqp://localhost")
	t.Setenv("SLURM_API_URL", "http://localhost/")
	t.Setenv("SLURM_JWT_TOKEN", "token")
	t.Setenv("SLURM_JOB_ID", "12345")
	t.Setenv("POLL_INTERVAL", "2s")
	t.Setenv("POLL_TIMEOUT", "5m")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ExecutionID != "exec-1" {
		t.Errorf("expected exec-1, got %s", cfg.ExecutionID)
	}
	if cfg.SlurmAPIURL != "http://localhost" {
		t.Errorf("expected trailing slash stripped, got %s", cfg.SlurmAPIURL)
	}
	if cfg.SlurmJobID != "12345" {
		t.Errorf("expected 12345, got %s", cfg.SlurmJobID)
	}
	if cfg.PollInterval != 2*time.Second {
		t.Errorf("expected 2s, got %v", cfg.PollInterval)
	}
	if cfg.PollTimeout != 5*time.Minute {
		t.Errorf("expected 5m, got %v", cfg.PollTimeout)
	}
	if cfg.SlurmAPIUser != "cloud-user" {
		t.Errorf("expected default SlurmAPIUser=cloud-user, got %s", cfg.SlurmAPIUser)
	}
}

func TestLoadConfig_CustomSlurmAPIUser(t *testing.T) {
	t.Setenv("EXECUTION_ID", "exec-1")
	t.Setenv("AMQP_URL", "amqp://localhost")
	t.Setenv("SLURM_API_URL", "http://localhost")
	t.Setenv("SLURM_JWT_TOKEN", "token")
	t.Setenv("SLURM_API_USER", "custom-user")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SlurmAPIUser != "custom-user" {
		t.Errorf("expected SlurmAPIUser=custom-user, got %s", cfg.SlurmAPIUser)
	}
}

func TestGetNodeState_SlurmHeaders(t *testing.T) {
	var receivedUser, receivedToken, receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUser = r.Header.Get("X-SLURM-USER-NAME")
		receivedToken = r.Header.Get("X-SLURM-USER-TOKEN")
		receivedPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]any{
			"nodes": []map[string]any{
				{"state": []string{"idle"}},
			},
		})
	}))
	defer server.Close()

	cfg := &agentConfig{
		SlurmAPIURL:  server.URL,
		SlurmJWT:     "test-jwt",
		SlurmAPIUser: "cloud-user",
	}

	_, _ = getNodeState(context.Background(), server.Client(), cfg, "gpu-01")

	if receivedUser != "cloud-user" {
		t.Errorf("expected X-SLURM-USER-NAME=cloud-user, got %q", receivedUser)
	}
	if receivedToken != "test-jwt" {
		t.Errorf("expected X-SLURM-USER-TOKEN=test-jwt, got %q", receivedToken)
	}
	if receivedPath != "/slurm/v0.0.40/node/gpu-01" {
		t.Errorf("expected v0.0.40 path, got %q", receivedPath)
	}
}

func TestDiscoverHostname(t *testing.T) {
	h := discoverHostname()
	if h == "" {
		t.Fatal("hostname should not be empty")
	}
	// Should not contain dots (domain stripped)
	if len(h) > 0 && h[0] == '.' {
		t.Errorf("hostname should not start with dot: %s", h)
	}
}

func TestPollDrainLoop_DetectsDrained(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var state string
		if calls < 3 {
			state = "idle"
		} else {
			state = "drained"
		}
		json.NewEncoder(w).Encode(map[string]any{
			"nodes": []map[string]any{
				{"state": []string{state}},
			},
		})
	}))
	defer server.Close()

	cfg := &agentConfig{
		SlurmAPIURL:  server.URL,
		SlurmJWT:     "test-token",
		PollInterval: 10 * time.Millisecond,
		PollTimeout:  5 * time.Second,
	}
	logger := newLogger("test-exec")
	client := server.Client()

	err := pollDrainLoop(context.Background(), client, cfg, "gpu-01", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls < 3 {
		t.Errorf("expected at least 3 calls, got %d", calls)
	}
}

func TestPollDrainLoop_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"nodes": []map[string]any{
				{"state": []string{"idle"}},
			},
		})
	}))
	defer server.Close()

	cfg := &agentConfig{
		SlurmAPIURL:  server.URL,
		SlurmJWT:     "test-token",
		PollInterval: 10 * time.Millisecond,
		PollTimeout:  50 * time.Millisecond,
	}
	logger := newLogger("test-exec")
	client := server.Client()

	err := pollDrainLoop(context.Background(), client, cfg, "gpu-01", logger)
	if err != errPollTimeout {
		t.Fatalf("expected poll timeout, got: %v", err)
	}
}

func TestPollDrainLoop_TransientError(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"nodes": []map[string]any{
				{"state": []string{"drained*"}},
			},
		})
	}))
	defer server.Close()

	cfg := &agentConfig{
		SlurmAPIURL:  server.URL,
		SlurmJWT:     "test-token",
		PollInterval: 10 * time.Millisecond,
		PollTimeout:  5 * time.Second,
	}
	logger := newLogger("test-exec")
	client := server.Client()

	err := pollDrainLoop(context.Background(), client, cfg, "gpu-01", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls < 2 {
		t.Errorf("expected at least 2 calls (first failed), got %d", calls)
	}
}

func TestDrainedStates(t *testing.T) {
	cases := []struct {
		state    string
		expected bool
	}{
		{"drained", true},
		{"drained*", true},
		{"down", true},
		{"down*", true},
		{"idle", false},
		{"allocated", false},
		{"draining", false},
	}
	for _, tc := range cases {
		if drainedStates[tc.state] != tc.expected {
			t.Errorf("drainedStates[%q] = %v, want %v", tc.state, drainedStates[tc.state], tc.expected)
		}
	}
}
