package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

type SwitchRequest struct {
	NodeName        string
	Direction       domain.SwitchDirection
	RequestedBy     string
	SlurmConstraint string
	SlurmPartition  string
}

var ErrInvalidSwitchRequest = errors.New("invalid switch request")

var ErrSwitchRequestDependency = errors.New("switch request dependency failure")

type EventPublisher interface {
	PublishRequested(ctx context.Context, executionID string, direction domain.SwitchDirection) error
	PublishNodeSelected(ctx context.Context, executionID, nodeName string) error
}

type SlurmNodeStateReader interface {
	GetNodeState(ctx context.Context, nodeName string) (*slurm.NodeState, error)
}

type SwitchService struct {
	store                store.Store
	publisher            EventPublisher
	logger               *slog.Logger
	slurmNodeStateReader SlurmNodeStateReader
}

func NewSwitchService(s store.Store, logger *slog.Logger, publishers ...EventPublisher) *SwitchService {
	var publisher EventPublisher
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	return &SwitchService{store: s, publisher: publisher, logger: trace.OrDefault(logger)}
}

func (s *SwitchService) WithSlurmNodeStateReader(reader SlurmNodeStateReader) *SwitchService {
	s.slurmNodeStateReader = reader
	return s
}

func (s *SwitchService) RequestSwitch(ctx context.Context, req SwitchRequest) (string, error) {
	var desired, previous domain.Owner
	currentState := domain.StateRequested
	nodeName := req.NodeName
	switch req.Direction {
	case domain.DirectionSlurmToOpenStack:
		if req.NodeName != "" {
			return "", fmt.Errorf("%w: node_name is not accepted for slurm_to_openstack", ErrInvalidSwitchRequest)
		}
		desired = domain.OwnerOpenStack
		previous = domain.OwnerSlurm
	case domain.DirectionOpenStackToSlurm:
		if req.NodeName == "" {
			return "", fmt.Errorf("%w: node_name is required for openstack_to_slurm", ErrInvalidSwitchRequest)
		}
		desired = domain.OwnerSlurm
		previous = domain.OwnerOpenStack
		currentState = domain.StateAwaitingTargetNode
	default:
		return "", fmt.Errorf("%w: invalid direction: %s", ErrInvalidSwitchRequest, req.Direction)
	}

	if req.Direction == domain.DirectionOpenStackToSlurm {
		if err := s.ensureOpenStackToSlurmRequestAllowed(ctx, req.NodeName); err != nil {
			return "", err
		}
	}

	id, err := generateID()
	if err != nil {
		return "", fmt.Errorf("generating execution id: %w", err)
	}

	now := time.Now()
	exec := &domain.Execution{
		ID:                       id,
		NodeName:                 nodeName,
		Direction:                req.Direction,
		RequestedBy:              req.RequestedBy,
		RequestedAt:              now,
		CurrentState:             currentState,
		DesiredOwner:             desired,
		PreviousOwner:            previous,
		StateVersion:             0,
		OverallStatus:            domain.OverallStatusActive,
		RequestedSlurmConstraint: req.SlurmConstraint,
		RequestedSlurmPartition:  req.SlurmPartition,
	}

	if err := s.store.CreateExecution(ctx, exec); err != nil {
		return "", fmt.Errorf("creating execution: %w", err)
	}
	if s.publisher != nil {
		s.publishAdmissionEvents(ctx, id, req)
	}

	s.logger.Info(trace.EventRequestAccepted,
		"execution_id", id,
		"node_name", nodeName,
		"direction", string(req.Direction),
		"requested_by", req.RequestedBy,
	)

	return id, nil
}

func (s *SwitchService) ensureOpenStackToSlurmRequestAllowed(ctx context.Context, nodeName string) error {
	if s.slurmNodeStateReader == nil {
		return nil
	}

	state, err := s.slurmNodeStateReader.GetNodeState(ctx, nodeName)
	if err != nil {
		return fmt.Errorf("%w: getting slurm node state for %s: %v", ErrSwitchRequestDependency, nodeName, err)
	}
	if state == nil {
		return fmt.Errorf("%w: slurm node state missing for %s", ErrSwitchRequestDependency, nodeName)
	}

	switch slurm.ClassifyAttachState(state.State) {
	case slurm.AttachStateResumeRequired:
		return nil
	case slurm.AttachStateReady:
		return fmt.Errorf("%w: node %s is already under Slurm ownership (state: %s)", ErrInvalidSwitchRequest, nodeName, state.State)
	default:
		return fmt.Errorf("%w: cannot determine Slurm ownership for node %s (state: %s)", ErrSwitchRequestDependency, nodeName, state.State)
	}
}

func (s *SwitchService) publishAdmissionEvents(ctx context.Context, executionID string, req SwitchRequest) {
	if err := s.publisher.PublishRequested(ctx, executionID, req.Direction); err != nil {
		s.logger.Warn("request.requested_publish_failed", "execution_id", executionID, "direction", string(req.Direction), "error", err.Error())
	}

	if req.Direction == domain.DirectionOpenStackToSlurm {
		if err := s.publisher.PublishNodeSelected(ctx, executionID, req.NodeName); err != nil {
			s.logger.Warn("request.node_selected_publish_failed", "execution_id", executionID, "direction", string(req.Direction), "node_name", req.NodeName, "error", err.Error())
		}
	}
}

type ExecutionStatus struct {
	ID            string
	NodeName      string
	Direction     domain.SwitchDirection
	CurrentState  domain.SwitchState
	StateVersion  int64
	OverallStatus domain.OverallStatus
	RequestedAt   time.Time
	ErrorCode     string
	ErrorSummary  string
}

func (s *SwitchService) GetStatus(ctx context.Context, executionID string) (*ExecutionStatus, error) {
	exec, err := s.store.GetExecution(ctx, executionID)
	if err != nil {
		return nil, err
	}
	return &ExecutionStatus{
		ID:            exec.ID,
		NodeName:      exec.NodeName,
		Direction:     exec.Direction,
		CurrentState:  exec.CurrentState,
		StateVersion:  exec.StateVersion,
		OverallStatus: exec.OverallStatus,
		RequestedAt:   exec.RequestedAt,
		ErrorCode:     exec.FinalErrorCode,
		ErrorSummary:  exec.FinalErrorSummary,
	}, nil
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
