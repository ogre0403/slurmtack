package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

type attachTestSlurmClient struct {
	nodeState         *slurm.NodeState
	getNodeStateCalls int
	resumeCalls       int
}

func (c *attachTestSlurmClient) SubmitPlaceholderJob(_ context.Context, _ slurm.PlaceholderJobRequest) (*slurm.PlaceholderJobResult, error) {
	return nil, nil
}

func (c *attachTestSlurmClient) GetNodeState(_ context.Context, _ string) (*slurm.NodeState, error) {
	c.getNodeStateCalls++
	return c.nodeState, nil
}

func (c *attachTestSlurmClient) DrainNode(_ context.Context, _, _ string) error { return nil }

func (c *attachTestSlurmClient) ResumeNode(_ context.Context, _ string) error {
	c.resumeCalls++
	return nil
}

func (c *attachTestSlurmClient) CancelJob(_ context.Context, _ string) error { return nil }

func TestDoAttachGuardsResumeForOpenStackToSlurm(t *testing.T) {
	tests := []struct {
		name            string
		state           string
		wantResumeCalls int
		wantErr         string
		wantState       domain.SwitchState
	}{
		{
			name:            "resumes composite drain state",
			state:           "drained+drain",
			wantResumeCalls: 1,
			wantState:       domain.StateTargetAttaching,
		},
		{
			name:            "skips already schedulable state",
			state:           "idle",
			wantResumeCalls: 0,
			wantState:       domain.StateTargetAttaching,
		},
		{
			name:            "fails unsupported state",
			state:           "fail",
			wantResumeCalls: 0,
			wantErr:         "node gpu-node-01 not attachable (state: fail)",
			wantState:       domain.StateHostReachable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			s := store.NewMemoryStore()
			exec := &domain.Execution{
				ID:            "exec-attach",
				NodeName:      "gpu-node-01",
				Direction:     domain.DirectionOpenStackToSlurm,
				RequestedBy:   "test",
				RequestedAt:   time.Now(),
				CurrentState:  domain.StateHostReachable,
				DesiredOwner:  domain.OwnerSlurm,
				PreviousOwner: domain.OwnerOpenStack,
				StateVersion:  1,
				OverallStatus: domain.OverallStatusActive,
			}
			if err := s.CreateExecution(ctx, exec); err != nil {
				t.Fatalf("CreateExecution() error = %v", err)
			}

			runner := engine.NewRunner(s, nil)
			sshRunner := &recordingSSHRunner{}
			client := &attachTestSlurmClient{
				nodeState: &slurm.NodeState{NodeName: exec.NodeName, State: tt.state},
			}
			orch := New(s, runner, sshRunner, client, nil, Config{}, nil)

			err := orch.doAttach(ctx, exec)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("doAttach() error = %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("doAttach() error = nil, want substring %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("doAttach() error = %q, want substring %q", err.Error(), tt.wantErr)
				}
			}

			if client.getNodeStateCalls != 1 {
				t.Fatalf("GetNodeState() calls = %d, want 1", client.getNodeStateCalls)
			}
			if client.resumeCalls != tt.wantResumeCalls {
				t.Fatalf("ResumeNode() calls = %d, want %d", client.resumeCalls, tt.wantResumeCalls)
			}
			if len(sshRunner.requests) != 2 {
				t.Fatalf("sshRunner requests = %d, want 2", len(sshRunner.requests))
			}
			for i, want := range []struct {
				action string
				step   string
			}{
				{action: "enable", step: "slurmd_enable"},
				{action: "start", step: "slurmd_start"},
			} {
				req := sshRunner.requests[i]
				if req.Host != exec.NodeName {
					t.Fatalf("request %d host = %q, want %q", i+1, req.Host, exec.NodeName)
				}
				if req.Command != "systemctl" {
					t.Fatalf("request %d command = %q, want %q", i+1, req.Command, "systemctl")
				}
				if len(req.Args) != 2 || req.Args[0] != want.action || req.Args[1] != "slurmd" {
					t.Fatalf("request %d args = %#v, want [%q %q]", i+1, req.Args, want.action, "slurmd")
				}
				if req.ExecutionID != exec.ID {
					t.Fatalf("request %d execution_id = %q, want %q", i+1, req.ExecutionID, exec.ID)
				}
				if req.StepName != want.step {
					t.Fatalf("request %d step_name = %q, want %q", i+1, req.StepName, want.step)
				}
				if req.Timeout != slurmdCommandTimeout {
					t.Fatalf("request %d timeout = %s, want %s", i+1, req.Timeout, slurmdCommandTimeout)
				}
			}

			updated, err := s.GetExecution(ctx, exec.ID)
			if err != nil {
				t.Fatalf("GetExecution() error = %v", err)
			}
			if updated.CurrentState != tt.wantState {
				t.Fatalf("CurrentState = %s, want %s", updated.CurrentState, tt.wantState)
			}
		})
	}
}

func TestDoAttachBlocksWhenSlurmdRestoreFails(t *testing.T) {
	ctx := context.Background()
	s := store.NewMemoryStore()
	exec := &domain.Execution{
		ID:            "exec-attach-fail",
		NodeName:      "gpu-node-01",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "test",
		RequestedAt:   time.Now(),
		CurrentState:  domain.StateHostReachable,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
		StateVersion:  1,
		OverallStatus: domain.OverallStatusActive,
	}
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution() error = %v", err)
	}

	runner := engine.NewRunner(s, nil)
	sshRunner := &scriptedSSHRunner{
		t: t,
		responses: []scriptedSSHResponse{{
			result: &remote.CommandResult{ExitCode: 1, Stderr: "permission denied"},
			err:    errors.New("permission denied"),
		}},
	}
	client := &attachTestSlurmClient{
		nodeState: &slurm.NodeState{NodeName: exec.NodeName, State: "drained"},
	}
	orch := New(s, runner, sshRunner, client, nil, Config{}, nil)

	err := orch.doAttach(ctx, exec)
	if err == nil {
		t.Fatal("doAttach() error = nil, want slurmd restore failure")
	}
	if !strings.Contains(err.Error(), "slurmd enable failed") {
		t.Fatalf("doAttach() error = %q, want substring %q", err.Error(), "slurmd enable failed")
	}
	if client.getNodeStateCalls != 0 {
		t.Fatalf("GetNodeState() calls = %d, want 0", client.getNodeStateCalls)
	}
	if client.resumeCalls != 0 {
		t.Fatalf("ResumeNode() calls = %d, want 0", client.resumeCalls)
	}
	if len(sshRunner.requests) != 1 {
		t.Fatalf("sshRunner requests = %d, want 1", len(sshRunner.requests))
	}

	updated, getErr := s.GetExecution(ctx, exec.ID)
	if getErr != nil {
		t.Fatalf("GetExecution() error = %v", getErr)
	}
	if updated.CurrentState != domain.StateHostReachable {
		t.Fatalf("CurrentState = %s, want %s", updated.CurrentState, domain.StateHostReachable)
	}
}
