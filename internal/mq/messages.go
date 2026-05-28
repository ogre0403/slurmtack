package mq

import "github.com/slurmtack/slurmtack/internal/domain"

type RequestedEvent struct {
	ExecutionID string                 `json:"execution_id"`
	Direction   domain.SwitchDirection `json:"direction"`
}

type NodeSelectedEvent struct {
	ExecutionID string `json:"execution_id"`
	NodeName    string `json:"node_name"`
}

type AllocationEvent struct {
	ExecutionID string `json:"execution_id"`
	JobID       string `json:"job_id"`
	NodeName    string `json:"node_name"`
}

type NodeDrainedEvent struct {
	ExecutionID string `json:"execution_id"`
	NodeName    string `json:"node_name"`
}
