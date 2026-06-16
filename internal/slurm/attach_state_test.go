package slurm

import (
	"context"
	"strings"
	"testing"

	"github.com/slurmtack/slurmtack/internal/domain"
)

type captureAttachClient struct {
	nodeState         *NodeState
	getNodeStateCalls int
	resumeCalls       int
}

func (c *captureAttachClient) SubmitPlaceholderJob(_ context.Context, _ PlaceholderJobRequest) (*PlaceholderJobResult, error) {
	return nil, nil
}

func (c *captureAttachClient) GetNodeState(_ context.Context, _ string) (*NodeState, error) {
	c.getNodeStateCalls++
	return c.nodeState, nil
}

func (c *captureAttachClient) GetNodeStateWithIdentity(_ context.Context, _ string, _ WorkloadIdentity) (*NodeState, error) {
	c.getNodeStateCalls++
	return c.nodeState, nil
}

func (c *captureAttachClient) DrainNode(_ context.Context, _, _ string) error { return nil }

func (c *captureAttachClient) ResumeNode(_ context.Context, _ string) error {
	c.resumeCalls++
	return nil
}

func (c *captureAttachClient) CancelJob(_ context.Context, _ string) error { return nil }

func (c *captureAttachClient) CancelJobWithIdentity(_ context.Context, _ string, _ WorkloadIdentity) error {
	return nil
}

func (c *captureAttachClient) ListPartitions(_ context.Context) ([]Partition, error) {
	return nil, nil
}

func (c *captureAttachClient) GetNodes(_ context.Context) ([]NodeState, error) {
	if c.nodeState != nil {
		return []NodeState{*c.nodeState}, nil
	}
	return nil, nil
}

func (c *captureAttachClient) VerifyToken(_ context.Context, _, _ string) error { return nil }

func TestAttachHandlerExecuteGuardedResume(t *testing.T) {
	exec := &domain.Execution{NodeName: "gpu-node-01"}
	tests := []struct {
		name            string
		state           string
		wantResumeCalls int
		wantErr         string
	}{
		{
			name:            "resumes composite drain state",
			state:           "idle+drain",
			wantResumeCalls: 1,
		},
		{
			name:            "skips already schedulable state",
			state:           "idle",
			wantResumeCalls: 0,
		},
		{
			name:            "fails unsupported state",
			state:           "fail",
			wantResumeCalls: 0,
			wantErr:         "node gpu-node-01 not attachable (state: fail)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &captureAttachClient{
				nodeState: &NodeState{NodeName: exec.NodeName, State: tt.state},
			}

			handler := NewAttachHandler(client)
			err := handler.Execute(context.Background(), exec)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Execute() error = %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("Execute() error = nil, want substring %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Execute() error = %q, want substring %q", err.Error(), tt.wantErr)
				}
			}

			if client.getNodeStateCalls != 1 {
				t.Fatalf("GetNodeState() calls = %d, want 1", client.getNodeStateCalls)
			}
			if client.resumeCalls != tt.wantResumeCalls {
				t.Fatalf("ResumeNode() calls = %d, want %d", client.resumeCalls, tt.wantResumeCalls)
			}
		})
	}
}
