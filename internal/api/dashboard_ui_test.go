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
		`id="execution-list"`,
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

func TestDashboardHTML_ExecutionPaginationControls(t *testing.T) {
	htmlPath := "../../docker/nginx/html/index.html"
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("reading dashboard HTML: %v", err)
	}
	html := string(content)

	paginationElements := []string{
		`id="exec-page-prev"`,
		`id="exec-page-next"`,
		`id="exec-page-info"`,
		`execPrevPage()`,
		`execNextPage()`,
	}
	for _, s := range paginationElements {
		if !strings.Contains(html, s) {
			t.Errorf("dashboard HTML missing pagination element: %s", s)
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

func TestDashboardJS_ExecutionPanelState(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	required := []string{
		"PAGE_SIZE",
		"execPage",
		"execPageCursors",
		"execHasMore",
		"selectedExecutionId",
		"loadExecutions",
		"renderExecutions",
		"execNextPage",
		"execPrevPage",
		"execution-list",
		"exec-page-prev",
		"exec-page-next",
		"exec-page-info",
	}
	for _, s := range required {
		if !strings.Contains(js, s) {
			t.Errorf("dashboard JS missing execution panel element: %s", s)
		}
	}
}

func TestDashboardJS_ExecutionDetailStateFirst(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	// current_state should appear before direction in the detail HTML construction
	currentStateIdx := strings.Index(js, "exec-current-state")
	directionIdx := strings.Index(js, "<strong>Direction:</strong>")
	if currentStateIdx < 0 {
		t.Error("dashboard JS detail view should include exec-current-state for prominent state display")
	}
	if directionIdx < 0 {
		t.Error("dashboard JS detail view should include Direction field")
	}
	if currentStateIdx > directionIdx {
		t.Error("current state should appear before direction in execution detail view")
	}
}

func TestDashboardJS_PaginationResetsOnFilterChange(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	// Both filter change handlers should reset page and cursors before loading
	if !strings.Contains(js, "state.execPage = 0") {
		t.Error("dashboard JS should reset execPage to 0 when filters change")
	}
	if !strings.Contains(js, "state.execPageCursors = [null]") {
		t.Error("dashboard JS should reset execPageCursors when filters change")
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

func TestDashboardJS_InlineCancelForActiveExecutions(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	// Active rows must render a cancel button with stopPropagation so clicking it
	// doesn't also trigger row selection
	if !strings.Contains(js, "event.stopPropagation()") {
		t.Error("inline cancel button must call event.stopPropagation() to avoid triggering row selection")
	}
	if !strings.Contains(js, "exec-cancel") {
		t.Error("dashboard JS should render exec-cancel button for active execution rows")
	}
	if !strings.Contains(js, `overall_status === 'active'`) {
		t.Error("dashboard JS should gate inline cancel on overall_status === 'active'")
	}
}

func TestDashboardJS_CancelRefreshesPageAndDetail(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	// After cancel, the code should reload executions and re-open selected detail
	if !strings.Contains(js, "await loadExecutions(0)") {
		t.Error("cancelExecution should reload executions after successful cancel")
	}
	if !strings.Contains(js, "if (state.selectedExecutionId) openDetail(state.selectedExecutionId)") {
		t.Error("cancelExecution should refresh the selected execution detail after successful cancel")
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
