package slurm

import "context"

type PlaceholderJobRequest struct {
	ExecutionID string
	Constraint  string
	Partition   string
}

type PlaceholderJobResult struct {
	JobID string
}

type NodeState struct {
	NodeName   string
	State      string
	GRES       []string
	RunningJob []string
}

type Client interface {
	SubmitPlaceholderJob(ctx context.Context, req PlaceholderJobRequest) (*PlaceholderJobResult, error)
	GetNodeState(ctx context.Context, nodeName string) (*NodeState, error)
	DrainNode(ctx context.Context, nodeName, reason string) error
	ResumeNode(ctx context.Context, nodeName string) error
	CancelJob(ctx context.Context, jobID string) error
}
