//go:build integration

package slurm

import (
	"context"
	"os"
	"testing"
	"time"
)

func integrationClient(t *testing.T) *RestClient {
	t.Helper()
	apiURL := os.Getenv("SLURM_API_URL")
	token := os.Getenv("SLURM_JWT_TOKEN")
	if apiURL == "" || token == "" {
		t.Skip("SLURM_API_URL and SLURM_JWT_TOKEN must be set for integration tests")
	}
	return NewRestClient(apiURL, token)
}

func TestIntegration_SubmitAndCancelJob(t *testing.T) {
	client := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.SubmitPlaceholderJob(ctx, PlaceholderJobRequest{
		ExecutionID: "integration-test",
		Constraint:  os.Getenv("SLURM_TEST_CONSTRAINT"),
		Partition:   os.Getenv("SLURM_TEST_PARTITION"),
	})
	if err != nil {
		t.Fatalf("SubmitPlaceholderJob failed: %v", err)
	}
	if result.JobID == "" {
		t.Fatal("expected non-empty job ID")
	}
	t.Logf("submitted job %s", result.JobID)

	err = client.CancelJob(ctx, result.JobID)
	if err != nil {
		t.Fatalf("CancelJob failed: %v", err)
	}
	t.Logf("cancelled job %s", result.JobID)
}

func TestIntegration_GetNodeState(t *testing.T) {
	client := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	nodeName := os.Getenv("SLURM_TEST_NODE")
	if nodeName == "" {
		t.Skip("SLURM_TEST_NODE must be set")
	}

	state, err := client.GetNodeState(ctx, nodeName)
	if err != nil {
		t.Fatalf("GetNodeState failed: %v", err)
	}
	if state.NodeName != nodeName {
		t.Errorf("expected node name %s, got %s", nodeName, state.NodeName)
	}
	if state.State == "" {
		t.Error("expected non-empty state")
	}
	t.Logf("node %s: state=%s gres=%v jobs=%v", state.NodeName, state.State, state.GRES, state.RunningJob)
}

func TestIntegration_DrainAndResumeNode(t *testing.T) {
	client := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	nodeName := os.Getenv("SLURM_TEST_NODE")
	if nodeName == "" {
		t.Skip("SLURM_TEST_NODE must be set")
	}

	err := client.DrainNode(ctx, nodeName, "integration test")
	if err != nil {
		t.Fatalf("DrainNode failed: %v", err)
	}
	t.Logf("drained node %s", nodeName)

	state, err := client.GetNodeState(ctx, nodeName)
	if err != nil {
		t.Fatalf("GetNodeState after drain failed: %v", err)
	}
	if state.State != "drained" && state.State != "drained+drain" && state.State != "idle+drain" {
		t.Logf("warning: expected drained state, got %s", state.State)
	}

	err = client.ResumeNode(ctx, nodeName)
	if err != nil {
		t.Fatalf("ResumeNode failed: %v", err)
	}
	t.Logf("resumed node %s", nodeName)

	state, err = client.GetNodeState(ctx, nodeName)
	if err != nil {
		t.Fatalf("GetNodeState after resume failed: %v", err)
	}
	t.Logf("node state after resume: %s", state.State)
}
