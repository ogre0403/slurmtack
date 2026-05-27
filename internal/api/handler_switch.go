package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slurmtack/slurmtack/internal/domain"
	"github.com/slurmtack/slurmtack/internal/service"
	"github.com/slurmtack/slurmtack/internal/store"
)

type SwitchHandler struct {
	svc   *service.SwitchService
	store store.Store
}

func NewSwitchHandler(svc *service.SwitchService, s store.Store) *SwitchHandler {
	return &SwitchHandler{svc: svc, store: s}
}

func (h *SwitchHandler) Create(c *gin.Context) {
	var req SwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	dir := domain.SwitchDirection(req.Direction)
	if dir != domain.DirectionSlurmToOpenStack && dir != domain.DirectionOpenStackToSlurm {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid direction: must be slurm_to_openstack or openstack_to_slurm"})
		return
	}

	id, err := h.svc.RequestSwitch(c.Request.Context(), service.SwitchRequest{
		NodeName:        req.NodeName,
		Direction:       dir,
		RequestedBy:     req.RequestedBy,
		SlurmConstraint: req.SlurmConstraint,
		SlurmPartition:  req.SlurmPartition,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, SwitchResponse{
		ExecutionID: id,
		StatusURL:   "/v1/switches/" + id,
	})
}

func (h *SwitchHandler) Get(c *gin.Context) {
	id := c.Param("id")
	exec, err := h.store.GetExecution(c.Request.Context(), id)
	if err == store.ErrNotFound {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "execution not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, ExecutionStatus{
		ID:            exec.ID,
		NodeName:      exec.NodeName,
		Direction:     string(exec.Direction),
		CurrentState:  string(exec.CurrentState),
		OverallStatus: string(exec.OverallStatus),
		RequestedAt:   exec.RequestedAt,
		RequestedBy:   exec.RequestedBy,
		ErrorCode:     exec.FinalErrorCode,
		ErrorSummary:  exec.FinalErrorSummary,
	})
}

func (h *SwitchHandler) List(c *gin.Context) {
	nodeFilter := c.Query("node")
	statusFilter := c.Query("status")

	executions, err := h.store.ListExecutions(c.Request.Context(), nodeFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	var results []ExecutionStatus
	for _, exec := range executions {
		if statusFilter != "" && string(exec.OverallStatus) != statusFilter {
			continue
		}
		results = append(results, ExecutionStatus{
			ID:            exec.ID,
			NodeName:      exec.NodeName,
			Direction:     string(exec.Direction),
			CurrentState:  string(exec.CurrentState),
			OverallStatus: string(exec.OverallStatus),
			RequestedAt:   exec.RequestedAt,
			RequestedBy:   exec.RequestedBy,
		})
	}

	if results == nil {
		results = []ExecutionStatus{}
	}
	c.JSON(http.StatusOK, results)
}

func (h *SwitchHandler) Cancel(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, ErrorResponse{Error: "not implemented"})
}
