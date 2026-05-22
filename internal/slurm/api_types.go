package slurm

type slurmError struct {
	Error string `json:"error"`
	Errno int    `json:"errno"`
}

type slurmErrorResponse struct {
	Errors []slurmError `json:"errors"`
}

type jobSubmitResponse struct {
	JobID  int            `json:"job_id"`
	Errors []slurmError   `json:"errors"`
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
