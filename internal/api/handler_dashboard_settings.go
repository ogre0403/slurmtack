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

func (h *DashboardSettingsHandler) Get(c *gin.Context) {
	configured := h.slurmSifPath != ""
	c.JSON(http.StatusOK, DashboardSettingsResponse{
		SlurmSifPathConfigured: configured,
		SlurmSifPath:           h.slurmSifPath,
	})
}
