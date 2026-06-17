package slurm

type slurmError struct {
	Error       string `json:"error"`
	Errno       int    `json:"errno"`
	ErrorNumber int    `json:"error_number,omitempty"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
}

type slurmErrorResponse struct {
	Errors []slurmError `json:"errors"`
}

type jobSubmitResponse struct {
	JobID  int          `json:"job_id"`
	Errors []slurmError `json:"errors"`
}

type nodeInfoResponse struct {
	Nodes  []nodeInfo   `json:"nodes"`
	Errors []slurmError `json:"errors"`
}

type nodeInfo struct {
	Name        string   `json:"name"`
	State       []string `json:"state"`
	Gres        string   `json:"gres"`
	GresUsed    string   `json:"gres_used"`
	AllocJobIDs []int    `json:"alloc_job_ids"`
}

type partitionsResponse struct {
	Partitions []partitionInfo `json:"partitions"`
	Errors     []slurmError    `json:"errors"`
}

type partitionInfo struct {
	Name  string         `json:"name"`
	Nodes partitionNodes `json:"nodes"`
}

type partitionNodes struct {
	Configured string `json:"configured"`
}

type jobStateResponse struct {
	Jobs   []jobInfo    `json:"jobs"`
	Errors []slurmError `json:"errors"`
}

type jobInfo struct {
	JobID    int      `json:"job_id"`
	JobState []string `json:"job_state"`
}
