package slurm

import (
	"context"
	"testing"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/store"
)

type captureAllocationClient struct {
	submitRequests []PlaceholderJobRequest
}

func (f *captureAllocationClient) SubmitPlaceholderJob(_ context.Context, req PlaceholderJobRequest) (*PlaceholderJobResult, error) {
	f.submitRequests = append(f.submitRequests, req)
	return &PlaceholderJobResult{JobID: "job-123"}, nil
}

func (f *captureAllocationClient) GetNodeState(_ context.Context, nodeName string) (*NodeState, error) {
	return nil, nil
}

func (f *captureAllocationClient) GetNodeStateWithIdentity(_ context.Context, _ string, _ WorkloadIdentity) (*NodeState, error) {
	return nil, nil
}

func (f *captureAllocationClient) DrainNode(_ context.Context, nodeName, reason string) error {
	return nil
}

func (f *captureAllocationClient) ResumeNode(_ context.Context, nodeName string) error {
	return nil
}

func (f *captureAllocationClient) CancelJob(_ context.Context, jobID string) error {
	return nil
}

func (f *captureAllocationClient) CancelJobWithIdentity(_ context.Context, _ string, _ WorkloadIdentity) error {
	return nil
}

func (f *captureAllocationClient) ListPartitions(_ context.Context) ([]Partition, error) {
	return nil, nil
}

func TestAllocationHandlerSubmitPlaceholderUsesRequestedPartition(t *testing.T) {
	fakeClient := &captureAllocationClient{}
	exec := &domain.Execution{
		ID:                       "exec-partition",
		Direction:                domain.DirectionSlurmToOpenStack,
		RequestedBy:              "operator",
		RequestedAt:              time.Now(),
		CurrentState:             domain.StateRequested,
		DesiredOwner:             domain.OwnerOpenStack,
		PreviousOwner:            domain.OwnerSlurm,
		OverallStatus:            domain.OverallStatusActive,
		RequestedSlurmConstraint: "gpu-a100",
		RequestedSlurmPartition:  "gpu-maint",
	}

	runAllocationSubmitPlaceholder(t, exec, fakeClient)

	if len(fakeClient.submitRequests) != 1 {
		t.Fatalf("submitRequests = %d, want 1", len(fakeClient.submitRequests))
	}
	if fakeClient.submitRequests[0].Partition != "gpu-maint" {
		t.Fatalf("Partition = %q, want gpu-maint", fakeClient.submitRequests[0].Partition)
	}
}

func TestAllocationHandlerSubmitPlaceholderLeavesPartitionEmptyWhenOmitted(t *testing.T) {
	fakeClient := &captureAllocationClient{}
	exec := &domain.Execution{
		ID:                       "exec-no-partition",
		Direction:                domain.DirectionSlurmToOpenStack,
		RequestedBy:              "operator",
		RequestedAt:              time.Now(),
		CurrentState:             domain.StateRequested,
		DesiredOwner:             domain.OwnerOpenStack,
		PreviousOwner:            domain.OwnerSlurm,
		OverallStatus:            domain.OverallStatusActive,
		RequestedSlurmConstraint: "gpu-a100",
	}

	runAllocationSubmitPlaceholder(t, exec, fakeClient)

	if len(fakeClient.submitRequests) != 1 {
		t.Fatalf("submitRequests = %d, want 1", len(fakeClient.submitRequests))
	}
	if fakeClient.submitRequests[0].Partition != "" {
		t.Fatalf("Partition = %q, want empty string", fakeClient.submitRequests[0].Partition)
	}
}

func runAllocationSubmitPlaceholder(t *testing.T, exec *domain.Execution, fakeClient *captureAllocationClient) {
	t.Helper()

	s := store.NewMemoryStore()
	ctx := context.Background()
	if err := s.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution() error = %v", err)
	}

	handler := NewAllocationHandler(s, engine.NewRunner(s, nil), fakeClient)
	if err := handler.SubmitPlaceholder(ctx, exec.ID); err != nil {
		t.Fatalf("SubmitPlaceholder() error = %v", err)
	}

	updated, err := s.GetExecution(ctx, exec.ID)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if updated.CurrentState != domain.StateAwaitingSourceAllocation {
		t.Fatalf("CurrentState = %s, want %s", updated.CurrentState, domain.StateAwaitingSourceAllocation)
	}
}
