package api

import "time"

type SwitchRequest struct {
	Direction         string `json:"direction" binding:"required"`
	RequestedBy       string `json:"requested_by" binding:"required"`
	NodeName          string `json:"node_name"`
	SlurmConstraint   string `json:"slurm_constraint"`
	SlurmPartition    string `json:"slurm_partition"`
	SlurmAccount      string `json:"slurm_account"`
	SlurmUser         string `json:"slurm_user"`
	SlurmUserToken    string `json:"slurm_user_token"`
	PlaceholderSIFFile string `json:"placeholder_sif_file"`
}

type SwitchResponse struct {
	ExecutionID string `json:"execution_id"`
	StatusURL   string `json:"status_url"`
}

type ExecutionStatus struct {
	ID            string    `json:"id"`
	NodeName      string    `json:"node_name"`
	Direction     string    `json:"direction"`
	CurrentState  string    `json:"current_state"`
	OverallStatus string    `json:"overall_status"`
	RequestedAt   time.Time `json:"requested_at"`
	RequestedBy   string    `json:"requested_by"`
	ErrorCode     string    `json:"error_code,omitempty"`
	ErrorSummary  string    `json:"error_summary,omitempty"`
}

type ExecutionDetail struct {
	ID                       string     `json:"id"`
	NodeName                 string     `json:"node_name"`
	Direction                string     `json:"direction"`
	CurrentState             string     `json:"current_state"`
	OverallStatus            string     `json:"overall_status"`
	RequestedAt              time.Time  `json:"requested_at"`
	RequestedBy              string     `json:"requested_by"`
	ErrorCode                string     `json:"error_code,omitempty"`
	ErrorSummary             string     `json:"error_summary,omitempty"`
	StateVersion             int64      `json:"state_version"`
	DesiredOwner             string     `json:"desired_owner"`
	PreviousOwner            string     `json:"previous_owner"`
	LockAcquiredAt           *time.Time `json:"lock_acquired_at,omitempty"`
	LockReleasedAt           *time.Time `json:"lock_released_at,omitempty"`
	RequestedSlurmConstraint string     `json:"requested_slurm_constraint,omitempty"`
	RequestedSlurmPartition  string     `json:"requested_slurm_partition,omitempty"`
	RequestedSlurmAccount    string     `json:"requested_slurm_account,omitempty"`
	PlaceholderJobID         string     `json:"placeholder_job_id,omitempty"`
	AllocationEventAt        *time.Time `json:"allocation_event_at,omitempty"`
	CancellationSourceState  string     `json:"cancellation_source_state,omitempty"`
}

type StepResponse struct {
	Sequence           int        `json:"sequence"`
	StepName           string     `json:"step_name"`
	Host               string     `json:"host"`
	StartedAt          time.Time  `json:"started_at"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	Status             string     `json:"status"`
	RetryCount         int        `json:"retry_count"`
	ExitCode           *int       `json:"exit_code,omitempty"`
	ErrorClass         string     `json:"error_class,omitempty"`
	CommandID          string     `json:"command_id,omitempty"`
	StdoutPath         string     `json:"stdout_path,omitempty"`
	StderrPath         string     `json:"stderr_path,omitempty"`
	SnapshotBeforePath string     `json:"snapshot_before_path,omitempty"`
	SnapshotAfterPath  string     `json:"snapshot_after_path,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type InventoryResponse struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Partitions  []InventoryPartition `json:"partitions"`
	Nodes       []InventoryNode      `json:"nodes"`
}

type InventoryPartition struct {
	Name  string   `json:"name"`
	Nodes []string `json:"nodes"`
}

type InventoryNode struct {
	NodeName           string                  `json:"node_name"`
	Partitions         []string                `json:"partitions"`
	Owner              string                  `json:"owner"`
	OwnerSource        string                  `json:"owner_source"`
	AvailableDirection string                  `json:"available_direction"`
	Slurm              *InventoryNodeSlurm     `json:"slurm,omitempty"`
	OpenStack          *InventoryNodeOpenStack  `json:"openstack,omitempty"`
	Switch             *InventoryNodeSwitch     `json:"switch,omitempty"`
	LastExecution      *InventoryLastExecution  `json:"last_execution,omitempty"`
}

type InventoryNodeSlurm struct {
	State       string   `json:"state"`
	GRES        []string `json:"gres"`
	RunningJobs []string `json:"running_jobs"`
}

type InventoryNodeOpenStack struct {
	ComputeService       InventoryComputeService `json:"compute_service"`
	InstanceCount        int                     `json:"instance_count"`
	ActiveMigrationCount int                     `json:"active_migration_count"`
}

type InventoryComputeService struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
	State   string `json:"state"`
}

type InventoryNodeSwitch struct {
	ActiveExecutionID string `json:"active_execution_id"`
	ActiveState       string `json:"active_state"`
}

type InventoryLastExecution struct {
	ID            string    `json:"id"`
	Direction     string    `json:"direction"`
	OverallStatus string    `json:"overall_status"`
	RequestedAt   time.Time `json:"requested_at"`
}
