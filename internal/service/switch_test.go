package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
)

type recordingEventPublisher struct {
	store                     store.Store
	requestedCalled           bool
	nodeSelectedCalled        bool
	executionID               string
	direction                 domain.SwitchDirection
	nodeName                  string
	sawPersisted              bool
	requestedReturnedError    error
	nodeSelectedReturnedError error
}

func (p *recordingEventPublisher) PublishRequested(ctx context.Context, executionID string, direction domain.SwitchDirection) error {
	p.requestedCalled = true
	p.executionID = executionID
	p.direction = direction
	_, err := p.store.GetExecution(ctx, executionID)
	p.sawPersisted = err == nil
	return p.requestedReturnedError
}

func (p *recordingEventPublisher) PublishNodeSelected(ctx context.Context, executionID, nodeName string) error {
	p.nodeSelectedCalled = true
	p.executionID = executionID
	p.nodeName = nodeName
	_, err := p.store.GetExecution(ctx, executionID)
	p.sawPersisted = err == nil
	return p.nodeSelectedReturnedError
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
		NodeName:    "gpu-01",
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
	if exec.NodeName != "gpu-01" {
		t.Fatalf("NodeName = %q, want gpu-01", exec.NodeName)
	}
}

func TestRequestSwitchRejectsOpenStackToSlurmWithoutNodeName(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil)

	_, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionOpenStackToSlurm,
		RequestedBy: "operator-1",
	})
	if !errors.Is(err, ErrInvalidSwitchRequest) {
		t.Fatalf("RequestSwitch() error = %v, want ErrInvalidSwitchRequest", err)
	}
}

func TestRequestSwitchPublishesRequestedEventAfterPersistence(t *testing.T) {
	s := store.NewMemoryStore()
	publisher := &recordingEventPublisher{store: s}
	svc := NewSwitchService(s, nil, publisher)

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionSlurmToOpenStack,
		RequestedBy: "operator-1",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}
	if !publisher.requestedCalled {
		t.Fatal("expected PublishRequested to be called")
	}
	if publisher.nodeSelectedCalled {
		t.Fatal("expected PublishNodeSelected not to be called")
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

func TestRequestSwitchPublishesNodeSelectedEventAfterPersistence(t *testing.T) {
	s := store.NewMemoryStore()
	publisher := &recordingEventPublisher{store: s}
	svc := NewSwitchService(s, nil, publisher)

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionOpenStackToSlurm,
		RequestedBy: "operator-1",
		NodeName:    "gpu-01",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}
	if !publisher.nodeSelectedCalled {
		t.Fatal("expected PublishNodeSelected to be called")
	}
	if !publisher.requestedCalled {
		t.Fatal("expected PublishRequested to be called")
	}
	if publisher.executionID != id {
		t.Fatalf("executionID = %q, want %q", publisher.executionID, id)
	}
	if publisher.nodeName != "gpu-01" {
		t.Fatalf("nodeName = %q, want gpu-01", publisher.nodeName)
	}
	if !publisher.sawPersisted {
		t.Fatal("expected PublishNodeSelected to observe persisted execution")
	}
}

func TestRequestSwitchLogsNodeSelectedPublishFailureWithoutFailingRequest(t *testing.T) {
	s := store.NewMemoryStore()
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	publisher := &recordingEventPublisher{store: s, nodeSelectedReturnedError: errors.New("publish failed")}
	svc := NewSwitchService(s, logger, publisher)

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionOpenStackToSlurm,
		RequestedBy: "operator-1",
		NodeName:    "gpu-01",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty execution id")
	}
	if _, getErr := s.GetExecution(context.Background(), id); getErr != nil {
		t.Fatalf("GetExecution() error = %v", getErr)
	}
	if !strings.Contains(logs.String(), "request.node_selected_publish_failed") {
		t.Fatalf("expected request.node_selected_publish_failed in logs, got %q", logs.String())
	}
}
