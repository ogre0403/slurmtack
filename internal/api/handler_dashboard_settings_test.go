package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupDashboardSettingsTest(t *testing.T, slurmSifPath string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewDashboardSettingsHandler(slurmSifPath)
	engine := gin.New()
	engine.GET("/v1/dashboard/settings", handler.Get)
	return engine
}

func TestDashboardSettings_ConfiguredSlurmSifPath(t *testing.T) {
	engine := setupDashboardSettingsTest(t, "slurmtack/build/output")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/dashboard/settings", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp DashboardSettingsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !resp.SlurmSifPathConfigured {
		t.Error("slurm_sif_path_configured should be true when path is set")
	}
	if resp.SlurmSifPath != "slurmtack/build/output" {
		t.Errorf("slurm_sif_path = %q, want %q", resp.SlurmSifPath, "slurmtack/build/output")
	}
}

func TestDashboardSettings_UnconfiguredSlurmSifPath(t *testing.T) {
	engine := setupDashboardSettingsTest(t, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/dashboard/settings", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp DashboardSettingsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.SlurmSifPathConfigured {
		t.Error("slurm_sif_path_configured should be false when path is not set")
	}
	if resp.SlurmSifPath != "" {
		t.Errorf("slurm_sif_path = %q, want empty string", resp.SlurmSifPath)
	}
}

func TestDashboardSettings_RequiresAuth(t *testing.T) {
	srv := setupTestServer(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/dashboard/settings", nil)
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", w.Code)
	}
}

func TestDashboardSettings_AuthorizedRequest(t *testing.T) {
	srv := NewServer(":0", nil, nil, nil, nil, WithJWTAuth(testJWTManager, nil), WithSlurmSifPath("my/sif/path"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/dashboard/settings", nil)
	req.Header.Set("Authorization", "Bearer "+testAuthToken(t))
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp DashboardSettingsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.SlurmSifPathConfigured {
		t.Error("expected slurm_sif_path_configured=true")
	}
	if resp.SlurmSifPath != "my/sif/path" {
		t.Errorf("slurm_sif_path = %q, want my/sif/path", resp.SlurmSifPath)
	}
}
