package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type DashboardSettingsHandler struct {
	slurmSifPath string
}

func NewDashboardSettingsHandler(slurmSifPath string) *DashboardSettingsHandler {
	return &DashboardSettingsHandler{slurmSifPath: slurmSifPath}
}

// Get returns dashboard configuration settings.
// @Summary     Get dashboard settings
// @Description Returns configuration values used by the Slurmtack dashboard, such as whether a Slurm SIF path is configured.
// @Tags        dashboard
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} DashboardSettingsResponse
// @Router      /v1/dashboard/settings [get]
func (h *DashboardSettingsHandler) Get(c *gin.Context) {
	configured := h.slurmSifPath != ""
	c.JSON(http.StatusOK, DashboardSettingsResponse{
		SlurmSifPathConfigured: configured,
		SlurmSifPath:           h.slurmSifPath,
	})
}
