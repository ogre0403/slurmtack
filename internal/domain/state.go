package domain

type SwitchState string

const (
	StateRequested                SwitchState = "requested"
	StateAwaitingSourceAllocation SwitchState = "awaiting_source_allocation"
	StateAwaitingTargetNode       SwitchState = "awaiting_target_node"
	StateNodeIdentified           SwitchState = "node_identified"
	StateLocked                   SwitchState = "locked"
	StatePrecheckPassed           SwitchState = "precheck_passed"
	StateSourceQuiescing          SwitchState = "source_quiescing"
	StateSourceDetached           SwitchState = "source_detached"
	StateHostReconfiguring        SwitchState = "host_reconfiguring"
	StateRebooting                SwitchState = "rebooting"
	StateHostReachable            SwitchState = "host_reachable"
	StateTargetAttaching          SwitchState = "target_attaching"
	StateVerifying                SwitchState = "verifying"
	StateCompleted                SwitchState = "completed"
	StateFailedNonDestructive     SwitchState = "failed_non_destructive"
	StateFailedNeedsRollback      SwitchState = "failed_needs_rollback"
	StateFailedManualRecovery     SwitchState = "failed_manual_recovery"
	StateCompensating             SwitchState = "compensating"
)

func (s SwitchState) IsTerminal() bool {
	switch s {
	case StateCompleted, StateFailedNonDestructive, StateFailedNeedsRollback, StateFailedManualRecovery:
		return true
	}
	return false
}

type SwitchDirection string

const (
	DirectionSlurmToOpenStack SwitchDirection = "slurm_to_openstack"
	DirectionOpenStackToSlurm SwitchDirection = "openstack_to_slurm"
)

type Owner string

const (
	OwnerSlurm     Owner = "slurm"
	OwnerOpenStack Owner = "openstack"
)

type FailureClass string

const (
	FailureTransient          FailureClass = "transient"
	FailurePrecheckBlocked    FailureClass = "precheck_blocked"
	FailureMutationPartial    FailureClass = "mutation_partial"
	FailureVerificationFailed FailureClass = "verification_failed"
	FailureUnknownAfterReboot FailureClass = "unknown_after_reboot"
)

type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusSucceeded StepStatus = "succeeded"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

type OverallStatus string

const (
	OverallStatusActive    OverallStatus = "active"
	OverallStatusSucceeded OverallStatus = "succeeded"
	OverallStatusFailed    OverallStatus = "failed"
)
