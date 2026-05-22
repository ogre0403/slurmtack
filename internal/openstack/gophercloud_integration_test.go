//go:build integration

package openstack

import (
	"context"
	"os"
	"testing"
)

func integrationClient(t *testing.T) Client {
	t.Helper()
	authURL := os.Getenv("OS_AUTH_URL")
	if authURL == "" {
		t.Skip("OS_AUTH_URL not set, skipping integration test")
	}

	domainName := os.Getenv("OS_USER_DOMAIN_NAME")
	if domainName == "" {
		domainName = "Default"
	}
	projectDomainName := os.Getenv("OS_PROJECT_DOMAIN_NAME")
	if projectDomainName == "" {
		projectDomainName = "Default"
	}

	client, err := NewGophecloudClient(context.Background(), AuthOpts{
		AuthURL:           authURL,
		Username:          os.Getenv("OS_USERNAME"),
		Password:          os.Getenv("OS_PASSWORD"),
		ProjectName:       os.Getenv("OS_PROJECT_NAME"),
		UserDomainName:    domainName,
		ProjectDomainName: projectDomainName,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client
}

func testHost(t *testing.T) string {
	t.Helper()
	host := os.Getenv("OS_TEST_HOST")
	if host == "" {
		t.Skip("OS_TEST_HOST not set, skipping integration test")
	}
	return host
}

func TestIntegrationListInstances(t *testing.T) {
	client := integrationClient(t)
	host := testHost(t)

	instances, err := client.ListInstances(context.Background(), host)
	if err != nil {
		t.Fatalf("ListInstances failed: %v", err)
	}

	for _, inst := range instances {
		if inst.ID == "" {
			t.Error("instance has empty ID")
		}
		if inst.Name == "" {
			t.Error("instance has empty Name")
		}
		if inst.Status == "" {
			t.Error("instance has empty Status")
		}
	}
}

func TestIntegrationGetComputeService(t *testing.T) {
	client := integrationClient(t)
	host := testHost(t)

	svc, err := client.GetComputeService(context.Background(), host)
	if err != nil {
		t.Fatalf("GetComputeService failed: %v", err)
	}

	if svc.Host != host {
		t.Errorf("expected host %s, got %s", host, svc.Host)
	}
	if svc.Status == "" {
		t.Error("service has empty Status")
	}
	if svc.State == "" {
		t.Error("service has empty State")
	}
}

func TestIntegrationDisableEnableComputeService(t *testing.T) {
	client := integrationClient(t)
	host := testHost(t)

	err := client.DisableComputeService(context.Background(), host, "integration-test")
	if err != nil {
		t.Fatalf("DisableComputeService failed: %v", err)
	}

	svc, err := client.GetComputeService(context.Background(), host)
	if err != nil {
		t.Fatalf("GetComputeService after disable failed: %v", err)
	}
	if svc.Enabled {
		t.Error("expected service to be disabled after DisableComputeService")
	}

	err = client.EnableComputeService(context.Background(), host)
	if err != nil {
		t.Fatalf("EnableComputeService failed: %v", err)
	}

	svc, err = client.GetComputeService(context.Background(), host)
	if err != nil {
		t.Fatalf("GetComputeService after enable failed: %v", err)
	}
	if !svc.Enabled {
		t.Error("expected service to be enabled after EnableComputeService")
	}
}

func TestIntegrationListActiveMigrations(t *testing.T) {
	client := integrationClient(t)
	host := testHost(t)

	ids, err := client.ListActiveMigrations(context.Background(), host)
	if err != nil {
		t.Fatalf("ListActiveMigrations failed: %v", err)
	}

	// We expect no active migrations on the test host typically, but just verify no error
	_ = ids
}
