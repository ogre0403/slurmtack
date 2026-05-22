package mq

type AllocationEvent struct {
	ExecutionID string `json:"execution_id"`
	JobID       string `json:"job_id"`
	NodeName    string `json:"node_name"`
}

type NodeDrainedEvent struct {
	ExecutionID string `json:"execution_id"`
	NodeName    string `json:"node_name"`
}
