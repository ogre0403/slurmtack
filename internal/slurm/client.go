package slurm

import "context"

type PlaceholderJobRequest struct {
	ExecutionID        string
	Constraint         string
	Partition          string
	Account            string
	WorkloadUser       string
	WorkloadToken      string
	PlaceholderSIFFile string
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

type Partition struct {
	Name  string
	Nodes []string
}

type WorkloadIdentity struct {
	User  string
	Token string
}

type Client interface {
	SubmitPlaceholderJob(ctx context.Context, req PlaceholderJobRequest) (*PlaceholderJobResult, error)
	GetNodeState(ctx context.Context, nodeName string) (*NodeState, error)
	GetNodeStateWithIdentity(ctx context.Context, nodeName string, id WorkloadIdentity) (*NodeState, error)
	GetNodes(ctx context.Context) ([]NodeState, error)
	DrainNode(ctx context.Context, nodeName, reason string) error
	ResumeNode(ctx context.Context, nodeName string) error
	CancelJob(ctx context.Context, jobID string) error
	CancelJobWithIdentity(ctx context.Context, jobID string, id WorkloadIdentity) error
	ListPartitions(ctx context.Context) ([]Partition, error)
	VerifyToken(ctx context.Context, user, token string) error
}
