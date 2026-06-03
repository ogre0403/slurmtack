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

func TestPollDrainLoop_ContextCancelled(t *testing.T) {
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
	}
	logger := newLogger("test-exec")
	client := server.Client()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := pollDrainLoop(ctx, client, cfg, "gpu-01", logger)
	if err == nil {
		t.Fatal("expected error when context cancelled, got nil")
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

func TestIsDrainComplete(t *testing.T) {
	cases := []struct {
		state    string
		expected bool
	}{
		// simple terminal states
		{"drained", true},
		{"drained*", true},
		{"down", true},
		{"down*", true},
		// composite states containing a drain token
		{"MIXED+DRAIN", true},
		{"mixed+drain", true},
		{"ALLOCATED+DRAIN", true},
		{"drained+drain", true},
		{"down+drain", true},
		// non-drain states
		{"idle", false},
		{"allocated", false},
		{"draining", false},
		{"MIXED", false},
	}
	for _, tc := range cases {
		got := isDrainComplete(tc.state)
		if got != tc.expected {
			t.Errorf("isDrainComplete(%q) = %v, want %v", tc.state, got, tc.expected)
		}
	}
}

// TestPollDrainLoop_MixedDrain is the regression test for slurm-14.out where the node
// stayed in MIXED+DRAIN but the old exact-match check never completed the poll loop.
func TestPollDrainLoop_MixedDrain(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var states []string
		if calls < 3 {
			states = []string{"MIXED"}
		} else {
			states = []string{"MIXED", "DRAIN"}
		}
		json.NewEncoder(w).Encode(map[string]any{
			"nodes": []map[string]any{
				{"state": states},
			},
		})
	}))
	defer server.Close()

	cfg := &agentConfig{
		SlurmAPIURL:  server.URL,
		SlurmJWT:     "test-token",
		PollInterval: 10 * time.Millisecond,
	}
	logger := newLogger("test-exec")

	err := pollDrainLoop(context.Background(), server.Client(), cfg, "gpu-01", logger)
	if err != nil {
		t.Fatalf("expected MIXED+DRAIN to complete poll loop, got error: %v", err)
	}
	if calls < 3 {
		t.Errorf("expected at least 3 calls, got %d", calls)
	}
}
