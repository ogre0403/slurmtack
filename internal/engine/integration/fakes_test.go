package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/slurmtack/slurmtack/internal/openstack"
	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/slurm"
)

type FakeSlurmClient struct {
	Nodes     map[string]*slurm.NodeState
	NextJobID string
	Drained   map[string]bool
}

func NewFakeSlurmClient() *FakeSlurmClient {
	return &FakeSlurmClient{
		Nodes:   make(map[string]*slurm.NodeState),
		Drained: make(map[string]bool),
	}
}

func (f *FakeSlurmClient) SubmitPlaceholderJob(_ context.Context, req slurm.PlaceholderJobRequest) (*slurm.PlaceholderJobResult, error) {
	return &slurm.PlaceholderJobResult{JobID: f.NextJobID}, nil
}

func (f *FakeSlurmClient) GetNodeState(_ context.Context, nodeName string) (*slurm.NodeState, error) {
	state, ok := f.Nodes[nodeName]
	if !ok {
		return nil, fmt.Errorf("node %s not found", nodeName)
	}
	return state, nil
}

func (f *FakeSlurmClient) GetNodeStateWithIdentity(_ context.Context, nodeName string, _ slurm.WorkloadIdentity) (*slurm.NodeState, error) {
	return f.GetNodeState(context.Background(), nodeName)
}

func (f *FakeSlurmClient) DrainNode(_ context.Context, nodeName, reason string) error {
	f.Drained[nodeName] = true
	if node, ok := f.Nodes[nodeName]; ok {
		node.State = "drained"
	}
	return nil
}

func (f *FakeSlurmClient) ResumeNode(_ context.Context, nodeName string) error {
	node, ok := f.Nodes[nodeName]
	if !ok {
		return fmt.Errorf("node %s not found", nodeName)
	}
	if slurm.ClassifyAttachState(node.State) != slurm.AttachStateResumeRequired {
		return fmt.Errorf("slurm_update error: Invalid node state specified")
	}

	f.Drained[nodeName] = false
	node.State = "idle"
	return nil
}

func (f *FakeSlurmClient) CancelJob(_ context.Context, jobID string) error {
	return nil
}

func (f *FakeSlurmClient) CancelJobWithIdentity(_ context.Context, jobID string, _ slurm.WorkloadIdentity) error {
	return nil
}

func (f *FakeSlurmClient) ListPartitions(_ context.Context) ([]slurm.Partition, error) {
	return nil, nil
}
func (f *FakeSlurmClient) GetNodes(_ context.Context) ([]slurm.NodeState, error) {
	var list []slurm.NodeState
	for _, n := range f.Nodes {
		if n != nil {
			list = append(list, *n)
		}
	}
	return list, nil
}

func (f *FakeSlurmClient) GetJobState(_ context.Context, _ string, _ slurm.WorkloadIdentity) (*slurm.JobState, error) {
	return nil, nil
}

func (f *FakeSlurmClient) VerifyToken(_ context.Context, _, _ string) error {
	return nil
}

type FakeOpenStackClient struct {
	Instances  map[string][]openstack.Instance
	Migrations map[string][]string
	Services   map[string]*openstack.ComputeServiceStatus
}

func NewFakeOpenStackClient() *FakeOpenStackClient {
	return &FakeOpenStackClient{
		Instances:  make(map[string][]openstack.Instance),
		Migrations: make(map[string][]string),
		Services:   make(map[string]*openstack.ComputeServiceStatus),
	}
}

func (f *FakeOpenStackClient) ListInstances(_ context.Context, hostName string) ([]openstack.Instance, error) {
	return f.Instances[hostName], nil
}

func (f *FakeOpenStackClient) ListActiveMigrations(_ context.Context, hostName string) ([]string, error) {
	return f.Migrations[hostName], nil
}

func (f *FakeOpenStackClient) GetComputeService(_ context.Context, hostName string) (*openstack.ComputeServiceStatus, error) {
	svc, ok := f.Services[hostName]
	if !ok {
		return &openstack.ComputeServiceStatus{Host: hostName, Enabled: false, State: "down"}, nil
	}
	return svc, nil
}

func (f *FakeOpenStackClient) DisableComputeService(_ context.Context, hostName, reason string) error {
	if svc, ok := f.Services[hostName]; ok {
		svc.Enabled = false
	}
	return nil
}

func (f *FakeOpenStackClient) EnableComputeService(_ context.Context, hostName string) error {
	if svc, ok := f.Services[hostName]; ok {
		svc.Enabled = true
		svc.State = "up"
	} else {
		f.Services[hostName] = &openstack.ComputeServiceStatus{Host: hostName, Enabled: true, State: "up"}
	}
	return nil
}

type FakeRemoteRunner struct {
	Results map[string]*remote.CommandResult
}

func NewFakeRemoteRunner() *FakeRemoteRunner {
	return &FakeRemoteRunner{Results: make(map[string]*remote.CommandResult)}
}

func (f *FakeRemoteRunner) Execute(_ context.Context, req remote.CommandRequest) (*remote.CommandResult, error) {
	key := req.Host + ":" + req.Command
	if result, ok := f.Results[key]; ok {
		return result, nil
	}
	return &remote.CommandResult{
		ExitCode: 0,
		Stdout:   "{}",
		Duration: 100 * time.Millisecond,
	}, nil
}
