package mq

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/orchestrator"
	"github.com/slurmtack/slurmtack/internal/service"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

type fakeAdmissionSlurmClient struct {
	submitRequests []slurm.PlaceholderJobRequest
}

func (f *fakeAdmissionSlurmClient) SubmitPlaceholderJob(_ context.Context, req slurm.PlaceholderJobRequest) (*slurm.PlaceholderJobResult, error) {
	f.submitRequests = append(f.submitRequests, req)
	return &slurm.PlaceholderJobResult{JobID: "job-123"}, nil
}

func (f *fakeAdmissionSlurmClient) GetNodeState(_ context.Context, nodeName string) (*slurm.NodeState, error) {
	return nil, nil
}

func (f *fakeAdmissionSlurmClient) GetNodeStateWithIdentity(_ context.Context, _ string, _ slurm.WorkloadIdentity) (*slurm.NodeState, error) {
	return nil, nil
}

func (f *fakeAdmissionSlurmClient) DrainNode(_ context.Context, nodeName, reason string) error {
	return nil
}

func (f *fakeAdmissionSlurmClient) ResumeNode(_ context.Context, nodeName string) error {
	return nil
}

func (f *fakeAdmissionSlurmClient) CancelJob(_ context.Context, jobID string) error {
	return nil
}

func (f *fakeAdmissionSlurmClient) CancelJobWithIdentity(_ context.Context, _ string, _ slurm.WorkloadIdentity) error {
	return nil
}

func (f *fakeAdmissionSlurmClient) ListPartitions(_ context.Context) ([]slurm.Partition, error) {
	return nil, nil
}
func (f *fakeAdmissionSlurmClient) GetNodes(_ context.Context) ([]slurm.NodeState, error) {
	return nil, nil
}

func (f *fakeAdmissionSlurmClient) GetJobState(_ context.Context, _ string, _ slurm.WorkloadIdentity) (*slurm.JobState, error) {
	return nil, nil
}

func (f *fakeAdmissionSlurmClient) VerifyToken(_ context.Context, _, _ string) error { return nil }

type ackRecorder struct {
	acked   int
	nacked  int
	requeue bool
}

type serviceDrivenPublisher struct {
	t        *testing.T
	consumer *Consumer
}

func (p *serviceDrivenPublisher) PublishRequested(ctx context.Context, executionID string, direction domain.SwitchDirection) error {
	p.t.Helper()
	ack := &ackRecorder{}
	p.consumer.handleRequested(ctx, newDelivery(p.t, RequestedEvent{
		ExecutionID: executionID,
		Direction:   direction,
	}, ack))
	if ack.acked != 1 || ack.nacked != 0 {
		p.t.Fatalf("requested publish acks = %d, nacks = %d, want 1 ack and 0 nacks", ack.acked, ack.nacked)
	}
	return nil
}

func (p *serviceDrivenPublisher) PublishNodeSelected(ctx context.Context, executionID, nodeName string) error {
	p.t.Helper()
	ack := &ackRecorder{}
	p.consumer.handleNodeSelected(ctx, newDelivery(p.t, NodeSelectedEvent{
		ExecutionID: executionID,
		NodeName:    nodeName,
	}, ack))
	if ack.acked != 1 || ack.nacked != 0 {
		p.t.Fatalf("node-selected publish acks = %d, nacks = %d, want 1 ack and 0 nacks", ack.acked, ack.nacked)
	}
	return nil
}

func (a *ackRecorder) Ack(uint64, bool) error {
	a.acked++
	return nil
}

func (a *ackRecorder) Nack(uint64, bool, bool) error {
	a.nacked++
	return nil
}

func (a *ackRecorder) Reject(uint64, bool) error {
	return nil
}

func newTestConsumer(s store.Store) *Consumer {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewConsumer(nil, s, logger)
}

func newDelivery(t *testing.T, payload any, ack *ackRecorder) amqp.Delivery {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return amqp.Delivery{Acknowledger: ack, DeliveryTag: 1, Body: body}
}

func seedExecution(t *testing.T, s store.Store, exec *domain.Execution) {
	t.Helper()
	if exec.RequestedAt.IsZero() {
		exec.RequestedAt = time.Now()
	}
	if exec.OverallStatus == "" {
		exec.OverallStatus = domain.OverallStatusActive
	}
	if err := s.CreateExecution(context.Background(), exec); err != nil {
		t.Fatalf("create execution: %v", err)
	}
}

func TestHandleRequestedAcceptsSlurmToOpenStackRequestedEvent(t *testing.T) {
	s := store.NewMemoryStore()
	seedExecution(t, s, &domain.Execution{
		ID:            "exec-requested-s2o",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "operator",
		CurrentState:  domain.StateRequested,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
	})

	ack := &ackRecorder{}
	consumer := newTestConsumer(s)
	consumer.handleRequested(context.Background(), newDelivery(t, RequestedEvent{
		ExecutionID: "exec-requested-s2o",
		Direction:   domain.DirectionSlurmToOpenStack,
	}, ack))

	if ack.acked != 1 || ack.nacked != 0 {
		t.Fatalf("acks = %d, nacks = %d, want 1 ack and 0 nacks", ack.acked, ack.nacked)
	}
}

func TestHandleRequestedAcceptsOpenStackToSlurmAwaitingTargetNode(t *testing.T) {
	s := store.NewMemoryStore()
	seedExecution(t, s, &domain.Execution{
		ID:            "exec-requested-o2s",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "operator",
		CurrentState:  domain.StateAwaitingTargetNode,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
	})

	ack := &ackRecorder{}
	consumer := newTestConsumer(s)
	consumer.handleRequested(context.Background(), newDelivery(t, RequestedEvent{
		ExecutionID: "exec-requested-o2s",
		Direction:   domain.DirectionOpenStackToSlurm,
	}, ack))

	if ack.acked != 1 || ack.nacked != 0 {
		t.Fatalf("acks = %d, nacks = %d, want 1 ack and 0 nacks", ack.acked, ack.nacked)
	}
}

func TestHandleRequestedAcksStaleEvent(t *testing.T) {
	s := store.NewMemoryStore()
	seedExecution(t, s, &domain.Execution{
		ID:            "exec-stale-requested",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "operator",
		CurrentState:  domain.StateNodeIdentified,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
	})

	ack := &ackRecorder{}
	consumer := newTestConsumer(s)
	consumer.handleRequested(context.Background(), newDelivery(t, RequestedEvent{
		ExecutionID: "exec-stale-requested",
		Direction:   domain.DirectionSlurmToOpenStack,
	}, ack))

	if ack.acked != 1 || ack.nacked != 0 {
		t.Fatalf("acks = %d, nacks = %d, want 1 ack and 0 nacks", ack.acked, ack.nacked)
	}
}

func TestHandleRequestedAcksUnknownExecution(t *testing.T) {
	s := store.NewMemoryStore()
	ack := &ackRecorder{}
	consumer := newTestConsumer(s)
	consumer.handleRequested(context.Background(), newDelivery(t, RequestedEvent{
		ExecutionID: "missing",
		Direction:   domain.DirectionSlurmToOpenStack,
	}, ack))

	if ack.acked != 1 || ack.nacked != 0 {
		t.Fatalf("acks = %d, nacks = %d, want 1 ack and 0 nacks", ack.acked, ack.nacked)
	}
}

func TestHandleRequestedAdmitsSlurmToOpenStackWithoutPolling(t *testing.T) {
	s := store.NewMemoryStore()
	seedExecution(t, s, &domain.Execution{
		ID:            "exec-admit-requested",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "operator",
		CurrentState:  domain.StateRequested,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
	})

	runner := engine.NewRunner(s, nil)
	fakeSlurm := &fakeAdmissionSlurmClient{}
	orch := orchestrator.New(s, runner, nil, fakeSlurm, nil, orchestrator.Config{
		TickInterval:    10 * time.Millisecond,
		SSHPollInterval: time.Second,
		SSHPollTimeout:  time.Second,
	}, nil)

	ack := &ackRecorder{}
	consumer := NewConsumer(nil, s, nil, orch)
	consumer.handleRequested(context.Background(), newDelivery(t, RequestedEvent{
		ExecutionID: "exec-admit-requested",
		Direction:   domain.DirectionSlurmToOpenStack,
	}, ack))

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		updated, err := s.GetExecution(context.Background(), "exec-admit-requested")
		if err != nil {
			t.Fatalf("GetExecution() error = %v", err)
		}
		if updated.CurrentState == domain.StateAwaitingSourceAllocation {
			if len(fakeSlurm.submitRequests) != 1 {
				t.Fatalf("submitRequests = %d, want 1", len(fakeSlurm.submitRequests))
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("timed out waiting for requested event admission to submit placeholder")
}

func TestHandleNodeSelectedTransitionsAwaitingTargetNode(t *testing.T) {
	s := store.NewMemoryStore()
	seedExecution(t, s, &domain.Execution{
		ID:            "exec-node-selected",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "operator",
		CurrentState:  domain.StateAwaitingTargetNode,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
	})

	ack := &ackRecorder{}
	consumer := newTestConsumer(s)
	consumer.handleNodeSelected(context.Background(), newDelivery(t, NodeSelectedEvent{
		ExecutionID: "exec-node-selected",
		NodeName:    "gpu-01",
	}, ack))

	if ack.acked != 1 || ack.nacked != 0 {
		t.Fatalf("acks = %d, nacks = %d, want 1 ack and 0 nacks", ack.acked, ack.nacked)
	}

	updated, err := s.GetExecution(context.Background(), "exec-node-selected")
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if updated.NodeName != "gpu-01" {
		t.Fatalf("NodeName = %q, want gpu-01", updated.NodeName)
	}
	if updated.CurrentState != domain.StateNodeIdentified {
		t.Fatalf("CurrentState = %q, want %q", updated.CurrentState, domain.StateNodeIdentified)
	}
}

func TestHandleNodeSelectedAcksDuplicateEvent(t *testing.T) {
	s := store.NewMemoryStore()
	seedExecution(t, s, &domain.Execution{
		ID:            "exec-duplicate-node-selected",
		NodeName:      "gpu-01",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "operator",
		CurrentState:  domain.StateNodeIdentified,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
	})

	ack := &ackRecorder{}
	consumer := newTestConsumer(s)
	consumer.handleNodeSelected(context.Background(), newDelivery(t, NodeSelectedEvent{
		ExecutionID: "exec-duplicate-node-selected",
		NodeName:    "gpu-02",
	}, ack))

	if ack.acked != 1 || ack.nacked != 0 {
		t.Fatalf("acks = %d, nacks = %d, want 1 ack and 0 nacks", ack.acked, ack.nacked)
	}

	updated, err := s.GetExecution(context.Background(), "exec-duplicate-node-selected")
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if updated.NodeName != "gpu-01" {
		t.Fatalf("NodeName = %q, want gpu-01", updated.NodeName)
	}
}

func TestHandleNodeSelectedAcksStaleExecution(t *testing.T) {
	s := store.NewMemoryStore()
	seedExecution(t, s, &domain.Execution{
		ID:            "exec-stale-node-selected",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "operator",
		CurrentState:  domain.StateCompleted,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
		OverallStatus: domain.OverallStatusSucceeded,
	})

	ack := &ackRecorder{}
	consumer := newTestConsumer(s)
	consumer.handleNodeSelected(context.Background(), newDelivery(t, NodeSelectedEvent{
		ExecutionID: "exec-stale-node-selected",
		NodeName:    "gpu-03",
	}, ack))

	if ack.acked != 1 || ack.nacked != 0 {
		t.Fatalf("acks = %d, nacks = %d, want 1 ack and 0 nacks", ack.acked, ack.nacked)
	}
}

func TestHandleNodeSelectedAcksUnknownExecution(t *testing.T) {
	s := store.NewMemoryStore()
	ack := &ackRecorder{}
	consumer := newTestConsumer(s)
	consumer.handleNodeSelected(context.Background(), newDelivery(t, NodeSelectedEvent{
		ExecutionID: "missing",
		NodeName:    "gpu-04",
	}, ack))

	if ack.acked != 1 || ack.nacked != 0 {
		t.Fatalf("acks = %d, nacks = %d, want 1 ack and 0 nacks", ack.acked, ack.nacked)
	}
}

func TestHandleNodeSelectedAdmitsOpenStackToSlurmWithoutPolling(t *testing.T) {
	s := store.NewMemoryStore()
	seedExecution(t, s, &domain.Execution{
		ID:            "exec-admit-node-selected",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "operator",
		CurrentState:  domain.StateAwaitingTargetNode,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
	})

	runner := engine.NewRunner(s, nil)
	orch := orchestrator.New(s, runner, nil, nil, nil, orchestrator.Config{
		TickInterval:    10 * time.Millisecond,
		SSHPollInterval: time.Second,
		SSHPollTimeout:  time.Second,
	}, nil)

	ack := &ackRecorder{}
	consumer := NewConsumer(nil, s, nil, orch)
	consumer.handleNodeSelected(context.Background(), newDelivery(t, NodeSelectedEvent{
		ExecutionID: "exec-admit-node-selected",
		NodeName:    "gpu-02",
	}, ack))

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		updated, getErr := s.GetExecution(context.Background(), "exec-admit-node-selected")
		if getErr != nil {
			t.Fatalf("GetExecution() error = %v", getErr)
		}
		if updated.NodeName == "gpu-02" && updated.CurrentState != domain.StateAwaitingTargetNode {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("timed out waiting for node-selected event admission to advance execution")
}

func TestRequestSwitchPublishesNodeSelectedAndAdmitsWithoutManualRMQClient(t *testing.T) {
	s := store.NewMemoryStore()
	runner := engine.NewRunner(s, nil)
	orch := orchestrator.New(s, runner, nil, nil, nil, orchestrator.Config{
		TickInterval:    10 * time.Millisecond,
		SSHPollInterval: time.Second,
		SSHPollTimeout:  time.Second,
	}, nil)
	consumer := NewConsumer(nil, s, nil, orch)
	publisher := &serviceDrivenPublisher{t: t, consumer: consumer}
	svc := service.NewSwitchService(s, nil, publisher)

	execID, err := svc.RequestSwitch(context.Background(), service.SwitchRequest{
		Direction:   domain.DirectionOpenStackToSlurm,
		RequestedBy: "operator",
		NodeName:    "gpu-05",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		updated, getErr := s.GetExecution(context.Background(), execID)
		if getErr != nil {
			t.Fatalf("GetExecution() error = %v", getErr)
		}
		if updated.NodeName == "gpu-05" && updated.CurrentState != domain.StateAwaitingTargetNode {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("timed out waiting for service-published node-selected event admission to advance execution")
}

func TestHandleAllocation_ClosesWaitStep(t *testing.T) {
	s := store.NewMemoryStore()
	seedExecution(t, s, &domain.Execution{
		ID:            "exec-alloc-step",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "operator",
		CurrentState:  domain.StateAwaitingSourceAllocation,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
	})

	tracker := engine.NewStepTracker(s, nil)
	_, _ = tracker.StartStep(context.Background(), "exec-alloc-step", domain.StepWaitForSourceAllocation, "")

	ack := &ackRecorder{}
	consumer := newTestConsumer(s)
	consumer.handleAllocation(context.Background(), newDelivery(t, AllocationEvent{
		ExecutionID: "exec-alloc-step",
		JobID:       "job-1",
		NodeName:    "gpu-01",
	}, ack))

	if ack.acked != 1 {
		t.Fatalf("acks = %d, want 1", ack.acked)
	}

	steps, _ := s.ListSteps(context.Background(), "exec-alloc-step")
	if len(steps) == 0 {
		t.Fatal("expected at least 1 step")
	}
	if steps[0].Status != domain.StepStatusSucceeded {
		t.Errorf("wait step status = %q, want succeeded", steps[0].Status)
	}
	if steps[0].EndedAt == nil {
		t.Error("wait step ended_at should be set")
	}
}

func TestHandleDrained_ClosesWaitStep(t *testing.T) {
	s := store.NewMemoryStore()
	seedExecution(t, s, &domain.Execution{
		ID:            "exec-drained-step",
		NodeName:      "gpu-01",
		Direction:     domain.DirectionSlurmToOpenStack,
		RequestedBy:   "operator",
		CurrentState:  domain.StateSourceQuiescing,
		DesiredOwner:  domain.OwnerOpenStack,
		PreviousOwner: domain.OwnerSlurm,
		StateVersion:  4,
	})

	tracker := engine.NewStepTracker(s, nil)
	_, _ = tracker.StartStep(context.Background(), "exec-drained-step", domain.StepWaitForSourceDrain, "gpu-01")

	ack := &ackRecorder{}
	consumer := newTestConsumer(s)
	consumer.handleDrained(context.Background(), newDelivery(t, NodeDrainedEvent{
		ExecutionID: "exec-drained-step",
		NodeName:    "gpu-01",
	}, ack))

	if ack.acked != 1 {
		t.Fatalf("acks = %d, want 1", ack.acked)
	}

	steps, _ := s.ListSteps(context.Background(), "exec-drained-step")
	if len(steps) == 0 {
		t.Fatal("expected at least 1 step")
	}
	if steps[0].Status != domain.StepStatusSucceeded {
		t.Errorf("wait step status = %q, want succeeded", steps[0].Status)
	}
	if steps[0].EndedAt == nil {
		t.Error("wait step ended_at should be set")
	}
}

func TestHandleNodeSelected_ClosesWaitStep(t *testing.T) {
	s := store.NewMemoryStore()
	seedExecution(t, s, &domain.Execution{
		ID:            "exec-nodesel-step",
		Direction:     domain.DirectionOpenStackToSlurm,
		RequestedBy:   "operator",
		CurrentState:  domain.StateAwaitingTargetNode,
		DesiredOwner:  domain.OwnerSlurm,
		PreviousOwner: domain.OwnerOpenStack,
	})

	tracker := engine.NewStepTracker(s, nil)
	_, _ = tracker.StartStep(context.Background(), "exec-nodesel-step", domain.StepWaitForTargetNode, "")

	ack := &ackRecorder{}
	consumer := newTestConsumer(s)
	consumer.handleNodeSelected(context.Background(), newDelivery(t, NodeSelectedEvent{
		ExecutionID: "exec-nodesel-step",
		NodeName:    "gpu-02",
	}, ack))

	if ack.acked != 1 {
		t.Fatalf("acks = %d, want 1", ack.acked)
	}

	steps, _ := s.ListSteps(context.Background(), "exec-nodesel-step")
	if len(steps) == 0 {
		t.Fatal("expected at least 1 step")
	}
	if steps[0].Status != domain.StepStatusSucceeded {
		t.Errorf("wait step status = %q, want succeeded", steps[0].Status)
	}
	if steps[0].EndedAt == nil {
		t.Error("wait step ended_at should be set")
	}
}
