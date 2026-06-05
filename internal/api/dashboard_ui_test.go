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
