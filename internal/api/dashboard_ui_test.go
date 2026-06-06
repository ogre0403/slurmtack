package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestDashboardHTML_ContainsRequiredRegions(t *testing.T) {
	htmlPath := "../../docker/nginx/html/index.html"
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("reading dashboard HTML: %v", err)
	}
	html := string(content)

	required := []string{
		`id="health-badge"`,
		`id="partition-list"`,
		`id="node-grid"`,
		`id="partition-action-bar"`,
		`id="history-list"`,
		`id="detail-drawer"`,
		`id="loading-overlay"`,
		`dashboard.js`,
	}
	for _, s := range required {
		if !strings.Contains(html, s) {
			t.Errorf("dashboard HTML missing required element: %s", s)
		}
	}
}

func TestDashboardJS_Exists(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	required := []string{
		"/v1/dashboard/inventory",
		"/v1/switches",
		"/api/health",
		"switchNode",
		"cancelExecution",
		"openDetail",
	}
	for _, s := range required {
		if !strings.Contains(js, s) {
			t.Errorf("dashboard JS missing: %s", s)
		}
	}
}

func TestDashboardJS_SwitchPayloads(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if !strings.Contains(js, `direction: direction, node_name: nodeName, requested_by: requestedBy`) {
		t.Error("openstack_to_slurm payload should include direction, node_name, and requested_by")
	}
	if !strings.Contains(js, `direction: 'slurm_to_openstack', requested_by: requestedBy`) {
		t.Error("slurm_to_openstack payload should include direction and requested_by")
	}
	if !strings.Contains(js, `slurm_partition`) {
		t.Error("slurm_to_openstack should support slurm_partition")
	}
}

func TestDashboardJS_NoNodeScopedSlurmToOpenstack(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if strings.Contains(js, `switchFromPartition('`) {
		t.Error("node cards should not wire switchFromPartition with a node argument")
	}
	if !strings.Contains(js, `switchFromPartition()`) {
		t.Error("partition action bar should call switchFromPartition without arguments")
	}
}

func TestDashboardJS_PartitionActionBar(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if !strings.Contains(js, `renderPartitionActionBar`) {
		t.Error("dashboard JS should define renderPartitionActionBar")
	}
	if !strings.Contains(js, `partition-action-bar`) {
		t.Error("dashboard JS should reference partition-action-bar element")
	}
}

func TestDashboardJS_PartitionScopedPayloadLogic(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if !strings.Contains(js, `if (state.selectedPartition) body.slurm_partition = state.selectedPartition`) {
		t.Error("slurm_to_openstack should conditionally include slurm_partition based on selectedPartition")
	}
	if strings.Contains(js, `node_name`) && strings.Contains(js, `slurm_to_openstack`) {
		lines := strings.Split(js, "\n")
		for _, line := range lines {
			if strings.Contains(line, "slurm_to_openstack") && strings.Contains(line, "node_name") {
				t.Error("slurm_to_openstack payload should never include node_name")
			}
		}
	}
}

func TestHealthEndpoint_Failure(t *testing.T) {
	srv, _ := setupHistoryServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"status":"ok"`) {
		t.Errorf("expected healthy response, got: %s", body)
	}
}
