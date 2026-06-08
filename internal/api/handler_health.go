package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slurmtack/slurmtack/internal/store"
)

type HealthHandler struct {
	store *store.SQLiteStore
}

func NewHealthHandler(s *store.SQLiteStore) *HealthHandler {
	return &HealthHandler{store: s}
}

// Check returns the health status of the service.
// @Summary     Health check
// @Description Returns 200 if the service is healthy (database reachable), 503 otherwise.
// @Tags        health
// @Produce     json
// @Success     200 {object} HealthResponse
// @Failure     503 {object} HealthResponse
// @Router      /health [get]
func (h *HealthHandler) Check(c *gin.Context) {
	if err := h.store.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, HealthResponse{Status: "unhealthy", Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}
