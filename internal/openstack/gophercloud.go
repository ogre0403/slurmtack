package openstack

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gophercloud/gophercloud/v2"
	gc_openstack "github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/services"
)

type AuthOpts struct {
	AuthURL           string
	Username          string
	Password          string
	ProjectName       string
	UserDomainName    string
	ProjectDomainName string
}

type gophecloudClient struct {
	compute *gophercloud.ServiceClient
}

func NewGophecloudClient(ctx context.Context, opts AuthOpts) (Client, error) {
	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint: opts.AuthURL,
		Username:         opts.Username,
		Password:         opts.Password,
		DomainName:       opts.UserDomainName,
		TenantName:       opts.ProjectName,
		Scope: &gophercloud.AuthScope{
			ProjectName: opts.ProjectName,
			DomainName:  opts.ProjectDomainName,
		},
	}

	provider, err := gc_openstack.AuthenticatedClient(ctx, authOpts)
	if err != nil {
		return nil, fmt.Errorf("authenticating with keystone: %w", err)
	}

	compute, err := gc_openstack.NewComputeV2(provider, gophercloud.EndpointOpts{})
	if err != nil {
		return nil, fmt.Errorf("creating compute client: %w", err)
	}

	compute.Microversion = "2.53"

	return &gophecloudClient{compute: compute}, nil
}

func (c *gophecloudClient) ListInstances(ctx context.Context, hostName string) ([]Instance, error) {
	opts := servers.ListOpts{Host: hostName}
	allPages, err := servers.List(c.compute, opts).AllPages(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing instances on %s: %w", hostName, err)
	}

	allServers, err := servers.ExtractServers(allPages)
	if err != nil {
		return nil, fmt.Errorf("listing instances on %s: %w", hostName, err)
	}

	instances := make([]Instance, len(allServers))
	for i, s := range allServers {
		instances[i] = Instance{
			ID:     s.ID,
			Name:   s.Name,
			Status: s.Status,
		}
	}
	return instances, nil
}

func (c *gophecloudClient) ListActiveMigrations(ctx context.Context, hostName string) ([]string, error) {
	url := c.compute.ServiceURL("os-migrations") + "?host=" + hostName + "&status=running"
	var resp struct {
		Migrations []struct {
			ID int `json:"id"`
		} `json:"migrations"`
	}

	_, err := c.compute.Get(ctx, url, &resp, nil)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing active migrations on %s: %w", hostName, err)
	}

	ids := make([]string, len(resp.Migrations))
	for i, m := range resp.Migrations {
		ids[i] = fmt.Sprintf("%d", m.ID)
	}
	return ids, nil
}

func (c *gophecloudClient) GetComputeService(ctx context.Context, hostName string) (*ComputeServiceStatus, error) {
	opts := services.ListOpts{Host: hostName, Binary: "nova-compute"}
	allPages, err := services.List(c.compute, opts).AllPages(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting compute service on %s: %w", hostName, err)
	}

	allServices, err := services.ExtractServices(allPages)
	if err != nil {
		return nil, fmt.Errorf("getting compute service on %s: %w", hostName, err)
	}

	if len(allServices) == 0 {
		return nil, fmt.Errorf("getting compute service on %s: service not found", hostName)
	}

	svc := allServices[0]
	return &ComputeServiceStatus{
		Host:    svc.Host,
		Status:  svc.Status,
		State:   svc.State,
		Enabled: svc.Status == "enabled",
	}, nil
}

func (c *gophecloudClient) DisableComputeService(ctx context.Context, hostName, reason string) error {
	svcID, err := c.resolveServiceID(ctx, hostName)
	if err != nil {
		return fmt.Errorf("disabling compute service on %s: %w", hostName, err)
	}

	opts := services.UpdateOpts{
		Status:         services.ServiceDisabled,
		DisabledReason: reason,
	}
	result := services.Update(ctx, c.compute, svcID, opts)
	if result.Err != nil {
		return fmt.Errorf("disabling compute service on %s: %w", hostName, result.Err)
	}
	return nil
}

func (c *gophecloudClient) EnableComputeService(ctx context.Context, hostName string) error {
	svcID, err := c.resolveServiceID(ctx, hostName)
	if err != nil {
		return fmt.Errorf("enabling compute service on %s: %w", hostName, err)
	}

	opts := services.UpdateOpts{
		Status: services.ServiceEnabled,
	}
	result := services.Update(ctx, c.compute, svcID, opts)
	if result.Err != nil {
		return fmt.Errorf("enabling compute service on %s: %w", hostName, result.Err)
	}
	return nil
}

func (c *gophecloudClient) resolveServiceID(ctx context.Context, hostName string) (string, error) {
	opts := services.ListOpts{Host: hostName, Binary: "nova-compute"}
	allPages, err := services.List(c.compute, opts).AllPages(ctx)
	if err != nil {
		return "", fmt.Errorf("service not found: %w", err)
	}

	allServices, err := services.ExtractServices(allPages)
	if err != nil {
		return "", fmt.Errorf("service not found: %w", err)
	}

	if len(allServices) == 0 {
		return "", fmt.Errorf("service not found")
	}

	return allServices[0].ID, nil
}

func isNotFound(err error) bool {
	if responseErr, ok := err.(gophercloud.ErrUnexpectedResponseCode); ok {
		return responseErr.Actual == http.StatusNotFound
	}
	return false
}

var _ Client = (*gophecloudClient)(nil)
