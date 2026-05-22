//go:build integration

package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestIntegration_AgentLifecycle(t *testing.T) {
	// This test requires:
	// - RabbitMQ running at AMQP_URL
	// - slurmrestd running at SLURM_API_URL
	// - SLURM_JWT_TOKEN set
	// - EXECUTION_ID set to a valid execution in the daemon
	// Run with: go test -tags=integration ./cmd/placeholder-agent/

	amqpURL := os.Getenv("AMQP_URL")
	slurmAPIURL := os.Getenv("SLURM_API_URL")
	slurmJWT := os.Getenv("SLURM_JWT_TOKEN")
	executionID := os.Getenv("EXECUTION_ID")

	if amqpURL == "" || slurmAPIURL == "" || slurmJWT == "" || executionID == "" {
		t.Skip("integration test requires AMQP_URL, SLURM_API_URL, SLURM_JWT_TOKEN, EXECUTION_ID")
	}

	cmd := exec.Command("go", "run", "./cmd/placeholder-agent/")
	cmd.Env = append(os.Environ(),
		"EXECUTION_ID="+executionID,
		"AMQP_URL="+amqpURL,
		"SLURM_API_URL="+slurmAPIURL,
		"SLURM_JWT_TOKEN="+slurmJWT,
		"POLL_INTERVAL=2s",
		"POLL_TIMEOUT=5m",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("agent exited with error: %v", err)
	}
}
