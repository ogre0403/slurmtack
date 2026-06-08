package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/slurmtack/slurmtack/internal/config"
	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
	"github.com/slurmtack/slurmtack/internal/trace"
)

type SwitchRequest struct {
	NodeName           string
	Direction          domain.SwitchDirection
	RequestedBy        string
	SlurmConstraint    string
	SlurmPartition     string
	SlurmAccount       string
	SlurmUser          string
	SlurmUserToken     string
	PlaceholderSIFFile string
}

var ErrInvalidSwitchRequest = errors.New("invalid switch request")

var ErrSwitchRequestDependency = errors.New("switch request dependency failure")

var ErrCancelNotAllowed = errors.New("cancellation not allowed in current state")

type EventPublisher interface {
	PublishRequested(ctx context.Context, executionID string, direction domain.SwitchDirection) error
	PublishNodeSelected(ctx context.Context, executionID, nodeName string) error
}

type SlurmNodeStateReader interface {
	GetNodeState(ctx context.Context, nodeName string) (*slurm.NodeState, error)
}

type ExecutionIntake interface {
	AdmitExecution(ctx context.Context, executionID string)
}

type SlurmWorkloadDefaults struct {
	User  string
	Token string
}

type PlaceholderSIFDefaults struct {
	Path string
	File string
}

type SwitchService struct {
	store                   store.Store
	steps                   *engine.StepTracker
	publisher               EventPublisher
	intake                  ExecutionIntake
	logger                  *slog.Logger
	slurmNodeStateReader    SlurmNodeStateReader
	slurmWorkloadDefaults   *SlurmWorkloadDefaults
	placeholderSIFDefaults  *PlaceholderSIFDefaults
}

func NewSwitchService(s store.Store, logger *slog.Logger, publishers ...EventPublisher) *SwitchService {
	var publisher EventPublisher
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	l := trace.OrDefault(logger)
	return &SwitchService{store: s, steps: engine.NewStepTracker(s, l), publisher: publisher, logger: l}
}

func (s *SwitchService) WithExecutionIntake(intake ExecutionIntake) *SwitchService {
	s.intake = intake
	return s
}

func (s *SwitchService) WithSlurmWorkloadDefaults(user, token string) *SwitchService {
	if user != "" || token != "" {
		s.slurmWorkloadDefaults = &SlurmWorkloadDefaults{User: user, Token: token}
	}
	return s
}

func (s *SwitchService) WithPlaceholderSIFDefaults(path, file string) *SwitchService {
	if path != "" || file != "" {
		s.placeholderSIFDefaults = &PlaceholderSIFDefaults{Path: path, File: file}
	}
	return s
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

	// Validate pairwise override and resolve effective workload identity for slurm_to_openstack.
	var resolvedSlurmUser, resolvedSlurmToken string
	if req.Direction == domain.DirectionSlurmToOpenStack {
		if (req.SlurmUser != "") != (req.SlurmUserToken != "") {
			return "", fmt.Errorf("%w: slurm_user and slurm_user_token must be provided together", ErrInvalidSwitchRequest)
		}
		if req.SlurmUser != "" && req.SlurmUserToken != "" {
			resolvedSlurmUser = req.SlurmUser
			resolvedSlurmToken = req.SlurmUserToken
		} else if s.slurmWorkloadDefaults != nil && s.slurmWorkloadDefaults.Token != "" {
			resolvedSlurmUser = s.slurmWorkloadDefaults.User
			resolvedSlurmToken = s.slurmWorkloadDefaults.Token
		} else {
			return "", fmt.Errorf("%w: slurm workload user and token are required (provide slurm_user/slurm_user_token or configure daemon defaults)", ErrInvalidSwitchRequest)
		}
	}

	// Resolve effective placeholder SIF filename for slurm_to_openstack.
	var resolvedPlaceholderSIFFile string
	if req.Direction == domain.DirectionSlurmToOpenStack {
		if s.placeholderSIFDefaults == nil || s.placeholderSIFDefaults.Path == "" {
			return "", fmt.Errorf("%w: placeholder SIF path configuration is invalid or missing", ErrInvalidSwitchRequest)
		}
		if req.PlaceholderSIFFile != "" {
			if !config.IsValidPlaceholderSIFFile(req.PlaceholderSIFFile) {
				return "", fmt.Errorf("%w: placeholder_sif_file must be a simple filename", ErrInvalidSwitchRequest)
			}
			resolvedPlaceholderSIFFile = req.PlaceholderSIFFile
		} else if s.placeholderSIFDefaults.File != "" {
			resolvedPlaceholderSIFFile = s.placeholderSIFDefaults.File
		} else {
			return "", fmt.Errorf("%w: placeholder SIF filename is required (provide placeholder_sif_file or configure SLURM_SIF_FILE)", ErrInvalidSwitchRequest)
		}
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
		RequestedSlurmAccount:    req.SlurmAccount,
		SlurmWorkloadUser:        resolvedSlurmUser,
		SlurmWorkloadToken:       resolvedSlurmToken,
		PlaceholderSIFFile:       resolvedPlaceholderSIFFile,
	}

	if err := s.store.CreateExecution(ctx, exec); err != nil {
		return "", fmt.Errorf("creating execution: %w", err)
	}

	if req.Direction == domain.DirectionOpenStackToSlurm {
		_, _ = s.steps.StartStep(ctx, id, domain.StepWaitForTargetNode, nodeName)
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

var cancellableStates = map[domain.SwitchState]bool{
	domain.StateAwaitingTargetNode:       true,
	domain.StateAwaitingSourceAllocation: true,
	domain.StateSourceQuiescing:          true,
}

// CancelSwitch attempts to claim an execution for cancellation.
// Returns the execution ID if accepted (202), ErrNotFound if not found,
// or ErrCancelNotAllowed if the current state is not cancellable.
func (s *SwitchService) CancelSwitch(ctx context.Context, executionID string) error {
	exec, err := s.store.GetExecution(ctx, executionID)
	if err != nil {
		return err
	}

	// Idempotent: already claimed or finished
	if exec.CurrentState == domain.StateCancelling || exec.CurrentState == domain.StateCancelled {
		return nil
	}

	if !cancellableStates[exec.CurrentState] {
		return fmt.Errorf("%w: %s", ErrCancelNotAllowed, exec.CurrentState)
	}

	// Persist the source state before transitioning
	exec.CancellationSourceState = exec.CurrentState
	if err := s.store.UpdateExecution(ctx, exec); err != nil {
		return fmt.Errorf("recording cancellation source state: %w", err)
	}

	// Re-read to get the latest state_version after UpdateExecution
	exec, err = s.store.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("re-reading execution: %w", err)
	}

	if err := s.store.AdvanceState(ctx, executionID, exec.StateVersion, domain.StateCancelling); err != nil {
		return fmt.Errorf("transitioning to cancelling: %w", err)
	}

	_ = s.steps.CloseRunningStep(ctx, executionID, domain.StepStatusSkipped)

	s.logger.Info("cancel.accepted",
		"execution_id", executionID,
		"source_state", string(exec.CancellationSourceState),
	)

	if s.intake != nil {
		s.intake.AdmitExecution(ctx, executionID)
	}
	return nil
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
