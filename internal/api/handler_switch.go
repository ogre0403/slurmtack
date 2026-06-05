package api

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

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
	id, err := h.svc.RequestSwitch(c.Request.Context(), service.SwitchRequest{
		NodeName:        req.NodeName,
		Direction:       dir,
		RequestedBy:     req.RequestedBy,
		SlurmConstraint: req.SlurmConstraint,
		SlurmPartition:  req.SlurmPartition,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidSwitchRequest) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
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

	detail := ExecutionDetail{
		ID:                      exec.ID,
		NodeName:                exec.NodeName,
		Direction:               string(exec.Direction),
		CurrentState:            string(exec.CurrentState),
		OverallStatus:           string(exec.OverallStatus),
		RequestedAt:             exec.RequestedAt,
		RequestedBy:             exec.RequestedBy,
		ErrorCode:               exec.FinalErrorCode,
		ErrorSummary:            exec.FinalErrorSummary,
		StateVersion:            exec.StateVersion,
		DesiredOwner:            string(exec.DesiredOwner),
		PreviousOwner:           string(exec.PreviousOwner),
		LockAcquiredAt:          exec.LockAcquiredAt,
		LockReleasedAt:          exec.LockReleasedAt,
		RequestedSlurmConstraint: exec.RequestedSlurmConstraint,
		RequestedSlurmPartition:  exec.RequestedSlurmPartition,
		PlaceholderJobID:         exec.PlaceholderJobID,
		AllocationEventAt:        exec.AllocationEventAt,
		CancellationSourceState:  string(exec.CancellationSourceState),
	}
	c.JSON(http.StatusOK, detail)
}

func (h *SwitchHandler) List(c *gin.Context) {
	nodeFilter := c.Query("node")
	statusFilter := c.Query("status")
	directionFilter := c.Query("direction")
	limitStr := c.Query("limit")
	beforeStr := c.Query("before")

	executions, err := h.store.ListExecutions(c.Request.Context(), nodeFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	sort.Slice(executions, func(i, j int) bool {
		return executions[i].RequestedAt.After(executions[j].RequestedAt)
	})

	var beforeTime time.Time
	if beforeStr != "" {
		parsed, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid before timestamp"})
			return
		}
		beforeTime = parsed
	}

	limit := 0
	if limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed < 1 {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid limit"})
			return
		}
		limit = parsed
	}

	var results []ExecutionStatus
	for _, exec := range executions {
		if statusFilter != "" && string(exec.OverallStatus) != statusFilter {
			continue
		}
		if directionFilter != "" && string(exec.Direction) != directionFilter {
			continue
		}
		if !beforeTime.IsZero() && !exec.RequestedAt.Before(beforeTime) {
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
			ErrorCode:     exec.FinalErrorCode,
			ErrorSummary:  exec.FinalErrorSummary,
		})
		if limit > 0 && len(results) >= limit {
			break
		}
	}

	if results == nil {
		results = []ExecutionStatus{}
	}
	c.JSON(http.StatusOK, results)
}

func (h *SwitchHandler) Steps(c *gin.Context) {
	id := c.Param("id")

	_, err := h.store.GetExecution(c.Request.Context(), id)
	if err == store.ErrNotFound {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "execution not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	steps, err := h.store.ListSteps(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	results := make([]StepResponse, 0, len(steps))
	for _, s := range steps {
		results = append(results, StepResponse{
			Sequence:           s.Sequence,
			StepName:           s.StepName,
			Host:               s.Host,
			StartedAt:          s.StartedAt,
			EndedAt:            s.EndedAt,
			Status:             string(s.Status),
			RetryCount:         s.RetryCount,
			ExitCode:           s.ExitCode,
			ErrorClass:         string(s.ErrorClass),
			CommandID:          s.CommandID,
			StdoutPath:         s.StdoutPath,
			StderrPath:         s.StderrPath,
			SnapshotBeforePath: s.SnapshotBeforePath,
			SnapshotAfterPath:  s.SnapshotAfterPath,
		})
	}
	c.JSON(http.StatusOK, results)
}

func (h *SwitchHandler) Cancel(c *gin.Context) {
	id := c.Param("id")
	err := h.svc.CancelSwitch(c.Request.Context(), id)
	if err == nil {
		c.JSON(http.StatusAccepted, SwitchResponse{
			ExecutionID: id,
			StatusURL:   "/v1/switches/" + id,
		})
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "execution not found"})
		return
	}
	if errors.Is(err, service.ErrCancelNotAllowed) {
		c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
}
