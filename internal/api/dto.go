package api

import "time"

type SwitchRequest struct {
	Direction       string `json:"direction" binding:"required"`
	RequestedBy     string `json:"requested_by" binding:"required"`
	NodeName        string `json:"node_name"`
	SlurmConstraint string `json:"slurm_constraint"`
}

type SwitchResponse struct {
	ExecutionID string `json:"execution_id"`
	StatusURL   string `json:"status_url"`
}

type ExecutionStatus struct {
	ID            string     `json:"id"`
	NodeName      string     `json:"node_name"`
	Direction     string     `json:"direction"`
	CurrentState  string     `json:"current_state"`
	OverallStatus string     `json:"overall_status"`
	RequestedAt   time.Time  `json:"requested_at"`
	RequestedBy   string     `json:"requested_by"`
	ErrorCode     string     `json:"error_code,omitempty"`
	ErrorSummary  string     `json:"error_summary,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
