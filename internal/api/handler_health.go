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

func (h *HealthHandler) Check(c *gin.Context) {
	if err := h.store.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
