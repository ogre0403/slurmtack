package openstack

import "context"

type Instance struct {
	ID     string
	Name   string
	Status string
}

type ComputeServiceStatus struct {
	Host    string
	Status  string
	State   string
	Enabled bool
}

type Client interface {
	ListInstances(ctx context.Context, hostName string) ([]Instance, error)
	ListActiveMigrations(ctx context.Context, hostName string) ([]string, error)
	GetComputeService(ctx context.Context, hostName string) (*ComputeServiceStatus, error)
	DisableComputeService(ctx context.Context, hostName, reason string) error
	EnableComputeService(ctx context.Context, hostName string) error
}
