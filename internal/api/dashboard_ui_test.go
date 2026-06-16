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
		`dashboard-config.js`,
	}
	for _, s := range required {
		if !strings.Contains(html, s) {
			t.Errorf("dashboard HTML missing required element: %s", s)
		}
	}
}

func TestDashboardHTML_RuntimeConfigLoadedBeforeDashboardJS(t *testing.T) {
	htmlPath := "../../docker/nginx/html/index.html"
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("reading dashboard HTML: %v", err)
	}
	html := string(content)

	configIdx := strings.Index(html, "dashboard-config.js")
	jsIdx := strings.Index(html, `src="dashboard.js"`)
	if configIdx < 0 {
		t.Fatal("dashboard HTML should include dashboard-config.js script tag")
	}
	if jsIdx < 0 {
		t.Fatal("dashboard HTML should include dashboard.js script tag")
	}
	if configIdx > jsIdx {
		t.Error("dashboard-config.js must be loaded before dashboard.js")
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
	if !strings.Contains(js, `direction: 'slurm_to_openstack'`) {
		t.Error("slurm_to_openstack payload should include direction")
	}
	if !strings.Contains(js, `requested_by: requestedBy`) {
		t.Error("slurm_to_openstack payload should include requested_by")
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

	if !strings.Contains(js, `if (effectivePartition) body.slurm_partition = effectivePartition`) {
		t.Error("slurm_to_openstack should conditionally include slurm_partition based on effectivePartition")
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

func TestDashboardHTML_SlurmSettingsRegion(t *testing.T) {
	htmlPath := "../../docker/nginx/html/index.html"
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("reading dashboard HTML: %v", err)
	}
	html := string(content)

	required := []string{
		`id="slurm-settings-btn"`,
		`id="slurm-settings-panel"`,
		`id="slurm-token-input"`,
		`id="slurm-derived-user"`,
		`id="slurm-account-input"`,
		`id="slurm-sif-input"`,
		`id="slurm-settings-validation"`,
		`id="slurm-settings-save"`,
		`id="slurm-settings-clear"`,
		`toggleSlurmSettings()`,
	}
	for _, s := range required {
		if !strings.Contains(html, s) {
			t.Errorf("dashboard HTML missing Slurm settings element: %s", s)
		}
	}
}

func TestDashboardJS_SlurmSettingsPersistence(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	required := []string{
		"loadSlurmSettingsFromStorage",
		"saveSlurmSettings",
		"clearSlurmSettings",
		"sessionStorage.setItem(SLURM_TOKEN_KEY",
		"sessionStorage.removeItem(SLURM_TOKEN_KEY)",
		"localStorage.setItem(SLURM_ACCOUNT_KEY",
		"localStorage.setItem(SLURM_SIF_KEY",
		"localStorage.removeItem(SLURM_ACCOUNT_KEY)",
		"localStorage.removeItem(SLURM_SIF_KEY)",
	}
	for _, s := range required {
		if !strings.Contains(js, s) {
			t.Errorf("dashboard JS missing Slurm settings persistence hook: %s", s)
		}
	}
}

func TestDashboardJS_HybridStorageSplit(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if !strings.Contains(js, "sessionStorage.getItem(SLURM_TOKEN_KEY)") {
		t.Error("slurm_user_token should be loaded from sessionStorage")
	}
	if !strings.Contains(js, "localStorage.getItem(SLURM_ACCOUNT_KEY)") {
		t.Error("slurm_account should be loaded from localStorage")
	}
	if !strings.Contains(js, "localStorage.getItem(SLURM_SIF_KEY)") {
		t.Error("placeholder_sif_file should be loaded from localStorage")
	}
	if !strings.Contains(js, "sessionStorage.getItem(SENSITIVE_TOKEN_KEY)") {
		t.Error("slurmtack_token should be loaded from sessionStorage")
	}
}

func TestDashboardJS_SilentTokenRenewal(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	required := []string{
		"exchangeToken",
		"authFetch",
		"/v1/auth/login",
		"handleAuthFailure",
		"renewingToken",
	}
	for _, s := range required {
		if !strings.Contains(js, s) {
			t.Errorf("dashboard JS missing silent token renewal component: %s", s)
		}
	}

	if !strings.Contains(js, "Your Slurm Token has expired") {
		t.Error("dashboard JS should show expiry message on auth failure")
	}
	if !strings.Contains(js, "panel.classList.add('open')") {
		t.Error("dashboard JS should open settings panel on auth failure")
	}
}

func TestDashboardJS_SlurmSettingsBlocking(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if !strings.Contains(js, "getSlurmSettingsValidation") {
		t.Error("dashboard JS should define getSlurmSettingsValidation for blocking incomplete settings")
	}
	if !strings.Contains(js, "isSlurmSettingsComplete") {
		t.Error("dashboard JS should define isSlurmSettingsComplete")
	}
	if !strings.Contains(js, "Cannot start slurm_to_openstack") {
		t.Error("dashboard JS should show blocking message when Slurm settings are incomplete")
	}
}

func TestDashboardJS_SlurmSettingsPayloadFields(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	requiredFields := []string{
		"slurm_account: state.slurmSettings.slurm_account",
		"placeholder_sif_file: state.slurmSettings.placeholder_sif_file",
		"slurm_user: state.slurmDerivedUser",
		"slurm_user_token: state.slurmSettings.slurm_user_token",
	}
	for _, s := range requiredFields {
		if !strings.Contains(js, s) {
			t.Errorf("slurm_to_openstack payload missing field: %s", s)
		}
	}
}

func TestDashboardJS_JWTDecodeDerivedUser(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if !strings.Contains(js, "decodeSlurmUser") {
		t.Error("dashboard JS should define decodeSlurmUser for JWT payload decoding")
	}
	if !strings.Contains(js, "payload.sun || payload.username || payload.preferred_username || payload.sub") {
		t.Error("decodeSlurmUser should use documented claim precedence: sun, username, preferred_username, sub")
	}
}

func TestDashboardJS_RuntimeConfigRead(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if strings.Contains(js, "/v1/dashboard/settings") {
		t.Error("dashboard JS should not fetch /v1/dashboard/settings (replaced by runtime config)")
	}
	if !strings.Contains(js, "SLURMTACK_CONFIG") {
		t.Error("dashboard JS should read window.SLURMTACK_CONFIG for runtime settings")
	}
	if !strings.Contains(js, "loadDashboardSettings") {
		t.Error("dashboard JS should define loadDashboardSettings")
	}
	if !strings.Contains(js, "slurmSifPath") {
		t.Error("dashboard JS state should include slurmSifPath")
	}
	if !strings.Contains(js, "slurmSifPathConfigured") {
		t.Error("dashboard JS state should include slurmSifPathConfigured")
	}
	if !strings.Contains(js, "slurmCloudPartition") {
		t.Error("dashboard JS state should include slurmCloudPartition")
	}
}

func TestNginxEntrypoint_PublishesSlurmCloudPartition(t *testing.T) {
	entrypoint := "../../docker/nginx/docker-entrypoint.sh"
	content, err := os.ReadFile(entrypoint)
	if err != nil {
		t.Fatalf("reading entrypoint: %v", err)
	}
	script := string(content)

	if !strings.Contains(script, "SLURM_CLOUD_PARTITION") {
		t.Error("entrypoint should read SLURM_CLOUD_PARTITION env var")
	}
	if !strings.Contains(script, "slurmCloudPartition") {
		t.Error("entrypoint should publish slurmCloudPartition in dashboard-config.js")
	}
}

func TestDashboardJS_SifLocationHintComputation(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if !strings.Contains(js, "computeExpectedSifLocation") {
		t.Error("dashboard JS should define computeExpectedSifLocation")
	}
	if !strings.Contains(js, `'/home/' + user + '/' + sifPath + '/' + sifFile`) {
		t.Error("dashboard JS should assemble expected SIF path as /home/<user>/<sifPath>/<sifFile>")
	}
}

func TestDashboardJS_SifLocationHintGuidanceStates(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if !strings.Contains(js, "token-derived workload user is required") {
		t.Error("dashboard JS should show guidance when workload user is unresolvable from token")
	}
	if !strings.Contains(js, "daemon SLURM_SIF_PATH configuration is required") {
		t.Error("dashboard JS should show guidance when daemon SLURM_SIF_PATH is not configured")
	}
	if !strings.Contains(js, "slurm-sif-location-hint") {
		t.Error("dashboard JS should reference slurm-sif-location-hint element")
	}
}

func TestDashboardHTML_SifLocationHintElement(t *testing.T) {
	htmlPath := "../../docker/nginx/html/index.html"
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("reading dashboard HTML: %v", err)
	}
	html := string(content)

	if !strings.Contains(html, `id="slurm-sif-location-hint"`) {
		t.Error("dashboard HTML should contain slurm-sif-location-hint element")
	}
	if !strings.Contains(html, `sif-location-hint`) {
		t.Error("dashboard HTML should define sif-location-hint CSS class")
	}
}

func TestDashboardJS_LoadDashboardSettingsCalledOnInit(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	initIdx := strings.Index(js, "async function init()")
	if initIdx < 0 {
		t.Fatal("dashboard JS should have an init function")
	}
	callIdx := strings.Index(js[initIdx:], "loadDashboardSettings()")
	if callIdx < 0 {
		t.Error("dashboard JS init should call loadDashboardSettings()")
	}
}

func TestDashboardJS_SifInputTriggersHintRecompute(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if !strings.Contains(js, "onSlurmSifInput") {
		t.Error("dashboard JS should define onSlurmSifInput to recompute hint on filename change")
	}
}

func TestDashboardJS_SaveSlurmSettingsDoesNotFetchSettings(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	saveIdx := strings.Index(js, "saveSlurmSettings = async function")
	if saveIdx < 0 {
		t.Fatal("dashboard JS should define saveSlurmSettings")
	}
	// saveSlurmSettings should no longer call loadDashboardSettings since config is loaded at startup from runtime config
	nextFuncIdx := strings.Index(js[saveIdx+1:], "window.")
	var saveBody string
	if nextFuncIdx > 0 {
		saveBody = js[saveIdx : saveIdx+1+nextFuncIdx]
	} else {
		saveBody = js[saveIdx:]
	}
	if strings.Contains(saveBody, "loadDashboardSettings") {
		t.Error("saveSlurmSettings should not call loadDashboardSettings (runtime config is loaded once at startup)")
	}
}

func TestDashboardJS_SaveSlurmSettingsRefreshesSessionOnTokenChange(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	saveIdx := strings.Index(js, "saveSlurmSettings = async function")
	if saveIdx < 0 {
		t.Fatal("dashboard JS should define saveSlurmSettings")
	}
	nextFuncIdx := strings.Index(js[saveIdx+1:], "window.")
	var saveBody string
	if nextFuncIdx > 0 {
		saveBody = js[saveIdx : saveIdx+1+nextFuncIdx]
	} else {
		saveBody = js[saveIdx:]
	}

	required := []string{
		"var previousSlurmToken = sessionStorage.getItem(SLURM_TOKEN_KEY) || ''",
		"var tokenChanged = previousSlurmToken !== state.slurmSettings.slurm_user_token",
		"if (!state.slurmSettings.slurm_user_token || tokenChanged)",
		"sessionStorage.removeItem(SENSITIVE_TOKEN_KEY)",
		"if (state.slurmSettings.slurm_user_token && (tokenChanged || !state.token))",
	}
	for _, s := range required {
		if !strings.Contains(saveBody, s) {
			t.Errorf("saveSlurmSettings should refresh session auth on token change: %s", s)
		}
	}
}

func TestDashboardJS_RuntimeConfigLoadedBeforeToken(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if strings.Contains(js, "prefetchDashboardSettings") {
		t.Error("dashboard JS should not define prefetchDashboardSettings (replaced by synchronous runtime config)")
	}

	initIdx := strings.Index(js, "async function init()")
	if initIdx < 0 {
		t.Fatal("dashboard JS should have an init function")
	}
	initBody := js[initIdx:]
	loadIdx := strings.Index(initBody, "loadDashboardSettings()")
	tokenIdx := strings.Index(initBody, "ensureToken()")
	if loadIdx < 0 {
		t.Fatal("init should call loadDashboardSettings()")
	}
	if tokenIdx < 0 {
		t.Fatal("init should call ensureToken()")
	}
	if loadIdx > tokenIdx {
		t.Error("loadDashboardSettings() should be called before ensureToken() since it reads synchronous runtime config")
	}
}

func TestDashboardJS_StepTimelineRendering(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	required := []string{
		"step-header",
		"step-seq",
		"step-name",
		"step-meta",
		"step-error",
		"step-error-summary",
		"step-paths",
		"formatStepName",
		"calcDuration",
		"isWaitStep",
		"STEP_LABELS",
		"step-wait",
		"step-action",
		"step-active-wait",
	}
	for _, s := range required {
		if !strings.Contains(js, s) {
			t.Errorf("dashboard JS missing step timeline element: %s", s)
		}
	}
}

func TestDashboardJS_ErrorSummaryRendering(t *testing.T) {
	jsPath := "../../docker/nginx/html/dashboard.js"
	content, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("reading dashboard JS: %v", err)
	}
	js := string(content)

	if !strings.Contains(js, "s.error_summary") {
		t.Error("dashboard JS should check step error_summary field for rendering")
	}
	if !strings.Contains(js, "step-error-summary") {
		t.Error("dashboard JS should render error_summary with step-error-summary class")
	}
}

func TestDashboardHTML_ErrorSummaryStyle(t *testing.T) {
	htmlPath := "../../docker/nginx/html/index.html"
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("reading dashboard HTML: %v", err)
	}
	html := string(content)

	if !strings.Contains(html, ".step-timeline .step-error-summary") {
		t.Error("dashboard HTML should define CSS style for .step-error-summary")
	}
}

func TestDashboardHTML_StepTimelineStyles(t *testing.T) {
	htmlPath := "../../docker/nginx/html/index.html"
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("reading dashboard HTML: %v", err)
	}
	html := string(content)

	required := []string{
		".step-timeline .step-status.skipped",
		".step-timeline .step-meta",
		".step-timeline .step-name",
		".step-timeline .step-wait",
		"pulse-wait",
	}
	for _, s := range required {
		if !strings.Contains(html, s) {
			t.Errorf("dashboard HTML missing step style: %s", s)
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
