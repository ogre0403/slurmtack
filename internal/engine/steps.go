package engine

import (
	"context"

	"github.com/slurmtack/slurmtack/internal/domain"
)

type SourceQuiesceHandler interface {
	StepHandler
	Quiesce(ctx context.Context, exec *domain.Execution) error
}

type SourceDetachHandler interface {
	StepHandler
	Detach(ctx context.Context, exec *domain.Execution) error
}

type TargetAttachHandler interface {
	StepHandler
	Attach(ctx context.Context, exec *domain.Execution) error
}

type VerificationHandler interface {
	StepHandler
	Verify(ctx context.Context, exec *domain.Execution) error
}

type DirectionHandlers struct {
	SourceQuiesce SourceQuiesceHandler
	SourceDetach  SourceDetachHandler
	TargetAttach  TargetAttachHandler
	Verification  VerificationHandler
}
