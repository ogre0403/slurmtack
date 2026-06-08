package domain

const (
	StepSubmitPlaceholder       = "submit_placeholder"
	StepWaitForSourceAllocation = "wait_for_source_allocation"
	StepWaitForTargetNode       = "wait_for_target_node"
	StepAcquireLease            = "acquire_lease"
	StepPrecheck                = "precheck"
	StepQuiesceSource           = "quiesce_source"
	StepWaitForSourceDrain      = "wait_for_source_drain"
	StepVerifySourceQuiesce     = "verify_source_quiesce"
	StepReconfigureHost         = "reconfigure_host"
	StepReboot                  = "reboot"
	StepWaitForSSHReachability  = "wait_for_ssh_reachability"
	StepAttachTarget            = "attach_target"
	StepVerifyTarget            = "verify_target"
	StepCompleteExecution       = "complete_execution"
	StepCancelCleanup           = "cancel_cleanup"
)
