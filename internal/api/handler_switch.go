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

// Create initiates a node ownership switch.
// @Summary     Request a node ownership switch
// @Description Submits a request to switch a node between Slurm and OpenStack ownership. Returns immediately with an execution ID; poll the status URL for progress.
// @Tags        switches
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body     SwitchRequest  true "Switch parameters"
// @Success     202  {object} SwitchResponse
// @Failure     400  {object} ErrorResponse
// @Failure     500  {object} ErrorResponse
// @Router      /v1/switches [post]
func (h *SwitchHandler) Create(c *gin.Context) {
	var req SwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if username, exists := c.Get(ContextKeyUsername); exists {
		if u, ok := username.(string); ok && u != "admin" {
			req.RequestedBy = u
		}
	}

	dir := domain.SwitchDirection(req.Direction)
	id, err := h.svc.RequestSwitch(c.Request.Context(), service.SwitchRequest{
		NodeName:           req.NodeName,
		Direction:          dir,
		RequestedBy:        req.RequestedBy,
		SlurmConstraint:    req.SlurmConstraint,
		SlurmPartition:     req.SlurmPartition,
		SlurmAccount:       req.SlurmAccount,
		SlurmUser:          req.SlurmUser,
		SlurmUserToken:     req.SlurmUserToken,
		PlaceholderSIFFile: req.PlaceholderSIFFile,
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

// Get returns the full detail of a switch execution.
// @Summary     Get switch execution detail
// @Description Returns all fields of a switch execution, including state, timing, Slurm job details, and cancellation info.
// @Tags        switches
// @Produce     json
// @Security    BearerAuth
// @Param       id  path     string true "Execution ID"
// @Success     200 {object} ExecutionDetail
// @Failure     404 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /v1/switches/{id} [get]
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
		RequestedSlurmAccount:    exec.RequestedSlurmAccount,
		PlaceholderJobID:         exec.PlaceholderJobID,
		AllocationEventAt:        exec.AllocationEventAt,
		CancellationSourceState:  string(exec.CancellationSourceState),
	}
	c.JSON(http.StatusOK, detail)
}

// List returns a paginated list of switch executions.
// @Summary     List switch executions
// @Description Returns executions ordered by most-recent-first. Supports filtering by node name, status, direction, and a before-timestamp cursor.
// @Tags        switches
// @Produce     json
// @Security    BearerAuth
// @Param       node      query    string false "Filter by node name"
// @Param       status    query    string false "Filter by overall_status (active, completed, failed, cancelled)"
// @Param       direction query    string false "Filter by direction (slurm_to_openstack, openstack_to_slurm)"
// @Param       limit     query    int    false "Maximum number of results (default: all)"
// @Param       before    query    string false "Return only executions requested before this RFC3339 timestamp"
// @Success     200 {array}  ExecutionStatus
// @Failure     400 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /v1/switches [get]
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

// Steps returns the durable execution step timeline for a switch.
// @Summary     List steps for a switch execution
// @Description Returns the persisted step timeline for the given execution in ascending sequence order. Each step represents an orchestrator action or asynchronous wait boundary. Active executions include a running step (with no ended_at) indicating the current position. Completed, failed, and cancelled executions include all steps with final status and timing metadata.
// @Tags        switches
// @Produce     json
// @Security    BearerAuth
// @Param       id  path     string true "Execution ID"
// @Success     200 {array}  StepResponse
// @Failure     404 {object} ErrorResponse "Execution not found"
// @Failure     500 {object} ErrorResponse
// @Router      /v1/switches/{id}/steps [get]
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
			ErrorSummary:       s.ErrorSummary,
			CommandID:          s.CommandID,
			StdoutPath:         s.StdoutPath,
			StderrPath:         s.StderrPath,
			SnapshotBeforePath: s.SnapshotBeforePath,
			SnapshotAfterPath:  s.SnapshotAfterPath,
		})
	}
	c.JSON(http.StatusOK, results)
}

// Cancel requests cancellation of an in-progress switch.
// @Summary     Cancel a switch execution
// @Description Requests cancellation of an active switch. Returns 202 if the cancellation request was accepted, 409 if the execution is not in a cancellable state.
// @Tags        switches
// @Produce     json
// @Security    BearerAuth
// @Param       id  path     string true "Execution ID"
// @Success     202 {object} SwitchResponse
// @Failure     404 {object} ErrorResponse
// @Failure     409 {object} ErrorResponse
// @Failure     500 {object} ErrorResponse
// @Router      /v1/switches/{id}/cancel [post]
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
