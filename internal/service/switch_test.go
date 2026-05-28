package service

import (
	"context"
	"errors"
	"testing"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
)

type recordingRequestedEventPublisher struct {
	store         store.Store
	called        bool
	executionID   string
	direction     domain.SwitchDirection
	sawPersisted  bool
	returnedError error
}

func (p *recordingRequestedEventPublisher) PublishRequested(ctx context.Context, executionID string, direction domain.SwitchDirection) error {
	p.called = true
	p.executionID = executionID
	p.direction = direction
	_, err := p.store.GetExecution(ctx, executionID)
	p.sawPersisted = err == nil
	return p.returnedError
}

func TestRequestSwitchPersistsSlurmPartition(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil)

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:       domain.DirectionSlurmToOpenStack,
		RequestedBy:     "operator-1",
		SlurmConstraint: "gpu-a100",
		SlurmPartition:  "gpu-maint",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}

	exec, err := s.GetExecution(context.Background(), id)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if exec.RequestedSlurmConstraint != "gpu-a100" {
		t.Fatalf("RequestedSlurmConstraint = %q, want gpu-a100", exec.RequestedSlurmConstraint)
	}
	if exec.RequestedSlurmPartition != "gpu-maint" {
		t.Fatalf("RequestedSlurmPartition = %q, want gpu-maint", exec.RequestedSlurmPartition)
	}
}

func TestRequestSwitchLeavesSlurmPartitionEmptyWhenOmitted(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil)

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:       domain.DirectionSlurmToOpenStack,
		RequestedBy:     "operator-1",
		SlurmConstraint: "gpu-a100",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}

	exec, err := s.GetExecution(context.Background(), id)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if exec.RequestedSlurmPartition != "" {
		t.Fatalf("RequestedSlurmPartition = %q, want empty string", exec.RequestedSlurmPartition)
	}
}

func TestRequestSwitchOpenStackToSlurmStartsAwaitingTargetNode(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil)

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionOpenStackToSlurm,
		RequestedBy: "operator-1",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}

	exec, err := s.GetExecution(context.Background(), id)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if exec.CurrentState != domain.StateAwaitingTargetNode {
		t.Fatalf("CurrentState = %q, want %q", exec.CurrentState, domain.StateAwaitingTargetNode)
	}
	if exec.NodeName != "" {
		t.Fatalf("NodeName = %q, want empty string", exec.NodeName)
	}
}

func TestRequestSwitchRejectsOpenStackToSlurmNodeName(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil)

	_, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionOpenStackToSlurm,
		RequestedBy: "operator-1",
		NodeName:    "gpu-01",
	})
	if !errors.Is(err, ErrInvalidSwitchRequest) {
		t.Fatalf("RequestSwitch() error = %v, want ErrInvalidSwitchRequest", err)
	}
}

func TestRequestSwitchPublishesRequestedEventAfterPersistence(t *testing.T) {
	s := store.NewMemoryStore()
	publisher := &recordingRequestedEventPublisher{store: s}
	svc := NewSwitchService(s, nil, publisher)

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionSlurmToOpenStack,
		RequestedBy: "operator-1",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}
	if !publisher.called {
		t.Fatal("expected PublishRequested to be called")
	}
	if publisher.executionID != id {
		t.Fatalf("executionID = %q, want %q", publisher.executionID, id)
	}
	if publisher.direction != domain.DirectionSlurmToOpenStack {
		t.Fatalf("direction = %q, want %q", publisher.direction, domain.DirectionSlurmToOpenStack)
	}
	if !publisher.sawPersisted {
		t.Fatal("expected PublishRequested to observe persisted execution")
	}
}
