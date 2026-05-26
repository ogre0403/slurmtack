package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

type SwitchRequest struct {
	NodeName        string
	Direction       domain.SwitchDirection
	RequestedBy     string
	SlurmConstraint string
}

type SwitchService struct {
	store  store.Store
	logger *slog.Logger
}

func NewSwitchService(s store.Store, logger *slog.Logger) *SwitchService {
	return &SwitchService{store: s, logger: trace.OrDefault(logger)}
}

func (s *SwitchService) RequestSwitch(ctx context.Context, req SwitchRequest) (string, error) {
	id, err := generateID()
	if err != nil {
		return "", fmt.Errorf("generating execution id: %w", err)
	}

	var desired, previous domain.Owner
	switch req.Direction {
	case domain.DirectionSlurmToOpenStack:
		desired = domain.OwnerOpenStack
		previous = domain.OwnerSlurm
	case domain.DirectionOpenStackToSlurm:
		desired = domain.OwnerSlurm
		previous = domain.OwnerOpenStack
	default:
		return "", fmt.Errorf("invalid direction: %s", req.Direction)
	}

	now := time.Now()
	exec := &domain.Execution{
		ID:                       id,
		NodeName:                 req.NodeName,
		Direction:                req.Direction,
		RequestedBy:              req.RequestedBy,
		RequestedAt:              now,
		CurrentState:             domain.StateRequested,
		DesiredOwner:             desired,
		PreviousOwner:            previous,
		StateVersion:             0,
		OverallStatus:            domain.OverallStatusActive,
		RequestedSlurmConstraint: req.SlurmConstraint,
	}

	if err := s.store.CreateExecution(ctx, exec); err != nil {
		return "", fmt.Errorf("creating execution: %w", err)
	}

	s.logger.Info(trace.EventRequestAccepted,
		"execution_id", id,
		"node_name", req.NodeName,
		"direction", string(req.Direction),
		"requested_by", req.RequestedBy,
	)

	return id, nil
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
