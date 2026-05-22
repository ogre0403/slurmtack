package openstack

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	th "github.com/gophercloud/gophercloud/v2/testhelper"
	fake "github.com/gophercloud/gophercloud/v2/testhelper/client"
)

func setupClient() *gophecloudClient {
	return &gophecloudClient{compute: fake.ServiceClient()}
}

func TestListInstances(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/servers/detail", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		th.TestFormValues(t, r, map[string]string{"host": "gpu-node-01"})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"servers": [
				{"id": "aaa-111", "name": "vm-1", "status": "ACTIVE"},
				{"id": "bbb-222", "name": "vm-2", "status": "SHUTOFF"}
			]
		}`)
	})

	client := setupClient()
	instances, err := client.ListInstances(context.Background(), "gpu-node-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}
	if instances[0].ID != "aaa-111" || instances[0].Name != "vm-1" || instances[0].Status != "ACTIVE" {
		t.Errorf("unexpected first instance: %+v", instances[0])
	}
	if instances[1].ID != "bbb-222" {
		t.Errorf("unexpected second instance: %+v", instances[1])
	}
}

func TestListInstancesEmpty(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/servers/detail", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"servers": []}`)
	})

	client := setupClient()
	instances, err := client.ListInstances(context.Background(), "empty-host")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 0 {
		t.Fatalf("expected 0 instances, got %d", len(instances))
	}
}

func TestListActiveMigrations(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/os-migrations", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		if r.URL.Query().Get("host") != "gpu-node-01" {
			t.Errorf("expected host=gpu-node-01, got %s", r.URL.Query().Get("host"))
		}
		if r.URL.Query().Get("status") != "running" {
			t.Errorf("expected status=running, got %s", r.URL.Query().Get("status"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"migrations": [{"id": 101}, {"id": 202}]}`)
	})

	client := setupClient()
	ids, err := client.ListActiveMigrations(context.Background(), "gpu-node-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(ids))
	}
	if ids[0] != "101" || ids[1] != "202" {
		t.Errorf("unexpected migration IDs: %v", ids)
	}
}

func TestListActiveMigrations404(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/os-migrations", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	client := setupClient()
	ids, err := client.ListActiveMigrations(context.Background(), "gpu-node-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected empty slice on 404, got %v", ids)
	}
}

func TestGetComputeService(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/os-services", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		th.TestFormValues(t, r, map[string]string{
			"host":   "gpu-node-01",
			"binary": "nova-compute",
		})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"services": [
				{"id": 42, "host": "gpu-node-01", "binary": "nova-compute", "status": "enabled", "state": "up"}
			]
		}`)
	})

	client := setupClient()
	svc, err := client.GetComputeService(context.Background(), "gpu-node-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.Host != "gpu-node-01" {
		t.Errorf("expected host gpu-node-01, got %s", svc.Host)
	}
	if !svc.Enabled {
		t.Error("expected service to be enabled")
	}
	if svc.State != "up" {
		t.Errorf("expected state up, got %s", svc.State)
	}
}

func TestGetComputeServiceNotFound(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/os-services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"services": []}`)
	})

	client := setupClient()
	_, err := client.GetComputeService(context.Background(), "missing-host")
	if err == nil {
		t.Fatal("expected error for missing service")
	}
	if !strings.Contains(err.Error(), "service not found") {
		t.Errorf("expected 'service not found' in error, got: %v", err)
	}
}

func TestDisableComputeService(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/os-services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"services": [{"id": 42, "host": "gpu-node-01", "binary": "nova-compute", "status": "enabled", "state": "up"}]}`)
	})
	th.Mux.HandleFunc("/os-services/42", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "PUT")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"service": {"id": 42, "host": "gpu-node-01", "binary": "nova-compute", "status": "disabled", "state": "up"}}`)
	})

	client := setupClient()
	err := client.DisableComputeService(context.Background(), "gpu-node-01", "maintenance")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDisableComputeServiceAlreadyDisabled(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/os-services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"services": [{"id": 42, "host": "gpu-node-01", "binary": "nova-compute", "status": "disabled", "state": "up"}]}`)
	})
	th.Mux.HandleFunc("/os-services/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"service": {"id": 42, "host": "gpu-node-01", "binary": "nova-compute", "status": "disabled", "state": "up"}}`)
	})

	client := setupClient()
	err := client.DisableComputeService(context.Background(), "gpu-node-01", "maintenance")
	if err != nil {
		t.Fatalf("expected nil error for idempotent disable, got: %v", err)
	}
}

func TestEnableComputeService(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/os-services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"services": [{"id": 42, "host": "gpu-node-01", "binary": "nova-compute", "status": "disabled", "state": "up"}]}`)
	})
	th.Mux.HandleFunc("/os-services/42", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "PUT")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"service": {"id": 42, "host": "gpu-node-01", "binary": "nova-compute", "status": "enabled", "state": "up"}}`)
	})

	client := setupClient()
	err := client.EnableComputeService(context.Background(), "gpu-node-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnableComputeServiceAlreadyEnabled(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/os-services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"services": [{"id": 42, "host": "gpu-node-01", "binary": "nova-compute", "status": "enabled", "state": "up"}]}`)
	})
	th.Mux.HandleFunc("/os-services/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"service": {"id": 42, "host": "gpu-node-01", "binary": "nova-compute", "status": "enabled", "state": "up"}}`)
	})

	client := setupClient()
	err := client.EnableComputeService(context.Background(), "gpu-node-01")
	if err != nil {
		t.Fatalf("expected nil error for idempotent enable, got: %v", err)
	}
}

func TestListInstancesErrorWrapping(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/servers/detail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	client := setupClient()
	_, err := client.ListInstances(context.Background(), "gpu-node-01")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "listing instances") {
		t.Errorf("expected 'listing instances' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "gpu-node-01") {
		t.Errorf("expected hostname in error, got: %v", err)
	}
}

func TestDisableComputeServiceErrorWrapping(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/os-services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"services": []}`)
	})

	client := setupClient()
	err := client.DisableComputeService(context.Background(), "gpu-node-01", "test")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "disabling compute service") {
		t.Errorf("expected 'disabling compute service' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "gpu-node-01") {
		t.Errorf("expected hostname in error, got: %v", err)
	}
}

func TestEnableComputeServiceErrorWrapping(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/os-services", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"services": []}`)
	})

	client := setupClient()
	err := client.EnableComputeService(context.Background(), "gpu-node-01")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "enabling compute service") {
		t.Errorf("expected 'enabling compute service' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "gpu-node-01") {
		t.Errorf("expected hostname in error, got: %v", err)
	}
}

func TestGetComputeServiceErrorWrapping(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/os-services", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	client := setupClient()
	_, err := client.GetComputeService(context.Background(), "gpu-node-01")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "getting compute service") {
		t.Errorf("expected 'getting compute service' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "gpu-node-01") {
		t.Errorf("expected hostname in error, got: %v", err)
	}
}

func TestListActiveMigrationsErrorWrapping(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()

	th.Mux.HandleFunc("/os-migrations", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	client := setupClient()
	_, err := client.ListActiveMigrations(context.Background(), "gpu-node-01")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "listing active migrations") {
		t.Errorf("expected 'listing active migrations' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "gpu-node-01") {
		t.Errorf("expected hostname in error, got: %v", err)
	}
}
