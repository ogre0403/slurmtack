package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

type fakeSlurmNodeStateReader struct {
	nodeState *slurm.NodeState
	err       error
	calls     int
}

func (f *fakeSlurmNodeStateReader) GetNodeState(_ context.Context, _ string) (*slurm.NodeState, error) {
	f.calls++
	return f.nodeState, f.err
}

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
	svc := NewSwitchService(s, nil).WithSlurmWorkloadDefaults("cloud-user", "default-token").WithPlaceholderSIFDefaults("slurmtack/build/output", "placeholder-agent.sif")

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
	svc := NewSwitchService(s, nil).WithSlurmWorkloadDefaults("cloud-user", "default-token").WithPlaceholderSIFDefaults("slurmtack/build/output", "placeholder-agent.sif")

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

func TestRequestSwitchRejectsOpenStackToSlurmWhenNodeAlreadyOwnedBySlurm(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{name: "idle", state: "idle"},
		{name: "mixed", state: "mixed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewMemoryStore()
			publisher := &recordingEventPublisher{store: s}
			reader := &fakeSlurmNodeStateReader{nodeState: &slurm.NodeState{NodeName: "gpu-01", State: tt.state}}
			svc := NewSwitchService(s, nil, publisher).WithSlurmNodeStateReader(reader)

			_, err := svc.RequestSwitch(context.Background(), SwitchRequest{
				Direction:   domain.DirectionOpenStackToSlurm,
				RequestedBy: "operator-1",
				NodeName:    "gpu-01",
			})
			if !errors.Is(err, ErrInvalidSwitchRequest) {
				t.Fatalf("RequestSwitch() error = %v, want ErrInvalidSwitchRequest", err)
			}
			if !strings.Contains(err.Error(), "already under Slurm ownership") {
				t.Fatalf("RequestSwitch() error = %q, want already-under-Slurm message", err.Error())
			}
			if reader.calls != 1 {
				t.Fatalf("GetNodeState() calls = %d, want 1", reader.calls)
			}
			if publisher.requestedCalled || publisher.nodeSelectedCalled {
				t.Fatal("expected no admission events to be published")
			}

			executions, listErr := s.ListExecutions(context.Background(), "")
			if listErr != nil {
				t.Fatalf("ListExecutions() error = %v", listErr)
			}
			if len(executions) != 0 {
				t.Fatalf("execution count = %d, want 0", len(executions))
			}
		})
	}
}

func TestRequestSwitchAllowsResumableOpenStackToSlurmState(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{name: "drained", state: "drained"},
		{name: "mixed with drain token", state: "mixed+drain"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewMemoryStore()
			publisher := &recordingEventPublisher{store: s}
			reader := &fakeSlurmNodeStateReader{nodeState: &slurm.NodeState{NodeName: "gpu-01", State: tt.state}}
			svc := NewSwitchService(s, nil, publisher).WithSlurmNodeStateReader(reader)

			id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
				Direction:   domain.DirectionOpenStackToSlurm,
				RequestedBy: "operator-1",
				NodeName:    "gpu-01",
			})
			if err != nil {
				t.Fatalf("RequestSwitch() error = %v", err)
			}
			if reader.calls != 1 {
				t.Fatalf("GetNodeState() calls = %d, want 1", reader.calls)
			}
			if !publisher.requestedCalled || !publisher.nodeSelectedCalled {
				t.Fatal("expected both admission events to be published")
			}

			exec, getErr := s.GetExecution(context.Background(), id)
			if getErr != nil {
				t.Fatalf("GetExecution() error = %v", getErr)
			}
			if exec.CurrentState != domain.StateAwaitingTargetNode {
				t.Fatalf("CurrentState = %q, want %q", exec.CurrentState, domain.StateAwaitingTargetNode)
			}
		})
	}
}

func TestRequestSwitchReturnsDependencyErrorWhenSlurmLookupFails(t *testing.T) {
	s := store.NewMemoryStore()
	publisher := &recordingEventPublisher{store: s}
	reader := &fakeSlurmNodeStateReader{err: errors.New("slurm unavailable")}
	svc := NewSwitchService(s, nil, publisher).WithSlurmNodeStateReader(reader)

	_, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionOpenStackToSlurm,
		RequestedBy: "operator-1",
		NodeName:    "gpu-01",
	})
	if !errors.Is(err, ErrSwitchRequestDependency) {
		t.Fatalf("RequestSwitch() error = %v, want ErrSwitchRequestDependency", err)
	}
	if !strings.Contains(err.Error(), "getting slurm node state for gpu-01") {
		t.Fatalf("RequestSwitch() error = %q, want lookup failure context", err.Error())
	}
	if reader.calls != 1 {
		t.Fatalf("GetNodeState() calls = %d, want 1", reader.calls)
	}
	if publisher.requestedCalled || publisher.nodeSelectedCalled {
		t.Fatal("expected no admission events to be published")
	}

	executions, listErr := s.ListExecutions(context.Background(), "")
	if listErr != nil {
		t.Fatalf("ListExecutions() error = %v", listErr)
	}
	if len(executions) != 0 {
		t.Fatalf("execution count = %d, want 0", len(executions))
	}
}

func TestRequestSwitchRejectsSlurmToOpenStackWithNodeName(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil)

	_, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionSlurmToOpenStack,
		RequestedBy: "operator-1",
		NodeName:    "gpu-01",
	})
	if !errors.Is(err, ErrInvalidSwitchRequest) {
		t.Fatalf("RequestSwitch() error = %v, want ErrInvalidSwitchRequest", err)
	}
	if !strings.Contains(err.Error(), "node_name is not accepted for slurm_to_openstack") {
		t.Fatalf("RequestSwitch() error = %q, want node_name rejection message", err.Error())
	}

	executions, listErr := s.ListExecutions(context.Background(), "")
	if listErr != nil {
		t.Fatalf("ListExecutions() error = %v", listErr)
	}
	if len(executions) != 0 {
		t.Fatalf("execution count = %d, want 0", len(executions))
	}
}

func TestRequestSwitchPublishesRequestedEventAfterPersistence(t *testing.T) {
	s := store.NewMemoryStore()
	publisher := &recordingEventPublisher{store: s}
	svc := NewSwitchService(s, nil, publisher).WithSlurmWorkloadDefaults("cloud-user", "default-token").WithPlaceholderSIFDefaults("slurmtack/build/output", "placeholder-agent.sif")

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

func TestRequestSwitchRejectsIncompleteCredentialOverride(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil).WithSlurmWorkloadDefaults("cloud-user", "default-token").WithPlaceholderSIFDefaults("slurmtack/build/output", "placeholder-agent.sif")

	_, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionSlurmToOpenStack,
		RequestedBy: "operator-1",
		SlurmUser:   "alice",
	})
	if !errors.Is(err, ErrInvalidSwitchRequest) {
		t.Fatalf("RequestSwitch() error = %v, want ErrInvalidSwitchRequest", err)
	}
	if !strings.Contains(err.Error(), "slurm_user and slurm_user_token must be provided together") {
		t.Fatalf("RequestSwitch() error = %q, want pairwise requirement message", err.Error())
	}
}

func TestRequestSwitchRejectsMissingEffectiveWorkloadIdentity(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil).WithPlaceholderSIFDefaults("slurmtack/build/output", "placeholder-agent.sif") // no workload defaults

	_, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionSlurmToOpenStack,
		RequestedBy: "operator-1",
	})
	if !errors.Is(err, ErrInvalidSwitchRequest) {
		t.Fatalf("RequestSwitch() error = %v, want ErrInvalidSwitchRequest", err)
	}
	if !strings.Contains(err.Error(), "slurm workload user and token are required") {
		t.Fatalf("RequestSwitch() error = %q, want workload identity requirement message", err.Error())
	}

	executions, _ := s.ListExecutions(context.Background(), "")
	if len(executions) != 0 {
		t.Fatalf("execution count = %d, want 0", len(executions))
	}
}

func TestRequestSwitchAcceptsRequestScopedCredentials(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil).WithPlaceholderSIFDefaults("slurmtack/build/output", "placeholder-agent.sif") // no daemon workload defaults needed

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:      domain.DirectionSlurmToOpenStack,
		RequestedBy:    "operator-1",
		SlurmUser:      "alice",
		SlurmUserToken: "jwt-123",
		SlurmAccount:   "proj-123",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}

	exec, err := s.GetExecution(context.Background(), id)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if exec.SlurmWorkloadUser != "alice" {
		t.Fatalf("SlurmWorkloadUser = %q, want alice", exec.SlurmWorkloadUser)
	}
	if exec.SlurmWorkloadToken != "jwt-123" {
		t.Fatalf("SlurmWorkloadToken = %q, want jwt-123", exec.SlurmWorkloadToken)
	}
	if exec.RequestedSlurmAccount != "proj-123" {
		t.Fatalf("RequestedSlurmAccount = %q, want proj-123", exec.RequestedSlurmAccount)
	}
}

func TestRequestSwitchFallsBackToDaemonDefaults(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil).WithSlurmWorkloadDefaults("cloud-user", "daemon-token").WithPlaceholderSIFDefaults("slurmtack/build/output", "placeholder-agent.sif")

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionSlurmToOpenStack,
		RequestedBy: "operator-1",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}

	exec, err := s.GetExecution(context.Background(), id)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if exec.SlurmWorkloadUser != "cloud-user" {
		t.Fatalf("SlurmWorkloadUser = %q, want cloud-user", exec.SlurmWorkloadUser)
	}
	if exec.SlurmWorkloadToken != "daemon-token" {
		t.Fatalf("SlurmWorkloadToken = %q, want daemon-token", exec.SlurmWorkloadToken)
	}
}

func TestRequestSwitchUsesDefaultPlaceholderSIFFile(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil).
		WithSlurmWorkloadDefaults("cloud-user", "token").
		WithPlaceholderSIFDefaults("slurmtack/build/output", "placeholder-agent.sif")

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionSlurmToOpenStack,
		RequestedBy: "operator-1",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}

	exec, err := s.GetExecution(context.Background(), id)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if exec.PlaceholderSIFFile != "placeholder-agent.sif" {
		t.Fatalf("PlaceholderSIFFile = %q, want placeholder-agent.sif", exec.PlaceholderSIFFile)
	}
}

func TestRequestSwitchUsesRequestOverridePlaceholderSIFFile(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil).
		WithSlurmWorkloadDefaults("cloud-user", "token").
		WithPlaceholderSIFDefaults("slurmtack/build/output", "placeholder-agent.sif")

	id, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:          domain.DirectionSlurmToOpenStack,
		RequestedBy:        "operator-1",
		PlaceholderSIFFile: "placeholder-agent-debug.sif",
	})
	if err != nil {
		t.Fatalf("RequestSwitch() error = %v", err)
	}

	exec, err := s.GetExecution(context.Background(), id)
	if err != nil {
		t.Fatalf("GetExecution() error = %v", err)
	}
	if exec.PlaceholderSIFFile != "placeholder-agent-debug.sif" {
		t.Fatalf("PlaceholderSIFFile = %q, want placeholder-agent-debug.sif", exec.PlaceholderSIFFile)
	}
}

func TestRequestSwitchRejectsInvalidPlaceholderSIFFile(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "path traversal", file: "../other-user/agent.sif"},
		{name: "absolute path", file: "/etc/agent.sif"},
		{name: "contains slash", file: "subdir/agent.sif"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewMemoryStore()
			svc := NewSwitchService(s, nil).
				WithSlurmWorkloadDefaults("cloud-user", "token").
				WithPlaceholderSIFDefaults("slurmtack/build/output", "placeholder-agent.sif")

			_, err := svc.RequestSwitch(context.Background(), SwitchRequest{
				Direction:          domain.DirectionSlurmToOpenStack,
				RequestedBy:        "operator-1",
				PlaceholderSIFFile: tt.file,
			})
			if !errors.Is(err, ErrInvalidSwitchRequest) {
				t.Fatalf("RequestSwitch() error = %v, want ErrInvalidSwitchRequest", err)
			}
			if !strings.Contains(err.Error(), "placeholder_sif_file must be a simple filename") {
				t.Fatalf("RequestSwitch() error = %q, want simple filename message", err.Error())
			}
		})
	}
}

func TestRequestSwitchRejectsMissingPlaceholderSIFFile(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil).
		WithSlurmWorkloadDefaults("cloud-user", "token").
		WithPlaceholderSIFDefaults("slurmtack/build/output", "") // no default file

	_, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:   domain.DirectionSlurmToOpenStack,
		RequestedBy: "operator-1",
	})
	if !errors.Is(err, ErrInvalidSwitchRequest) {
		t.Fatalf("RequestSwitch() error = %v, want ErrInvalidSwitchRequest", err)
	}
	if !strings.Contains(err.Error(), "placeholder SIF filename is required") {
		t.Fatalf("RequestSwitch() error = %q, want filename requirement message", err.Error())
	}
}

func TestRequestSwitchRejectsMissingPlaceholderSIFPathConfig(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewSwitchService(s, nil).
		WithSlurmWorkloadDefaults("cloud-user", "token") // no placeholder SIF defaults

	_, err := svc.RequestSwitch(context.Background(), SwitchRequest{
		Direction:          domain.DirectionSlurmToOpenStack,
		RequestedBy:        "operator-1",
		PlaceholderSIFFile: "placeholder-agent.sif",
	})
	if !errors.Is(err, ErrInvalidSwitchRequest) {
		t.Fatalf("RequestSwitch() error = %v, want ErrInvalidSwitchRequest", err)
	}
	if !strings.Contains(err.Error(), "placeholder SIF path configuration is invalid or missing") {
		t.Fatalf("RequestSwitch() error = %q, want path config message", err.Error())
	}
}
