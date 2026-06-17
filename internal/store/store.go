package store

import (
	"context"
	"errors"

	"github.com/slurmtack/slurmtack/internal/domain"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrVersionConflict = errors.New("state version conflict")
	ErrLeaseHeld       = errors.New("lease already held by another execution")
	ErrLeaseNotHeld    = errors.New("lease not held by this execution")
)

type Store interface {
	CreateExecution(ctx context.Context, exec *domain.Execution) error
	GetExecution(ctx context.Context, id string) (*domain.Execution, error)
	ListExecutions(ctx context.Context, nodeName string) ([]*domain.Execution, error)
	ListActiveExecutions(ctx context.Context) ([]*domain.Execution, error)

	// AdvanceState transitions the execution to newState only if the current
	// state_version matches expectedVersion. On success, state_version is incremented.
	AdvanceState(ctx context.Context, executionID string, expectedVersion int64, newState domain.SwitchState) error

	// UpdateExecution persists field changes that are not state transitions
	// (e.g., binding node_name, recording placeholder_job_id).
	UpdateExecution(ctx context.Context, exec *domain.Execution) error

	AcquireLease(ctx context.Context, lease *domain.NodeLease) error
	ReleaseLease(ctx context.Context, nodeName string, executionID string) error
	GetLease(ctx context.Context, nodeName string) (*domain.NodeLease, error)

	CreateStep(ctx context.Context, step *domain.StepRecord) error
	UpdateStep(ctx context.Context, step *domain.StepRecord) error
	ListSteps(ctx context.Context, executionID string) ([]*domain.StepRecord, error)

	// RecordAdminTokenRenewal persists one audit record for an SSH-minted Slurm
	// admin token issuance. The record never contains the token material itself.
	RecordAdminTokenRenewal(ctx context.Context, renewal *domain.AdminTokenRenewal) error
	ListAdminTokenRenewals(ctx context.Context) ([]*domain.AdminTokenRenewal, error)
}
