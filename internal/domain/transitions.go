package domain

var allowedTransitions = map[SwitchState][]SwitchState{
	StateRequested:                {StateAwaitingSourceAllocation, StateAwaitingTargetNode, StateNodeIdentified, StateLocked, StateFailedNonDestructive},
	StateAwaitingSourceAllocation: {StateNodeIdentified, StateFailedNonDestructive, StateCancelling},
	StateAwaitingTargetNode:       {StateNodeIdentified, StateFailedNonDestructive, StateCancelling},
	StateNodeIdentified:           {StateLocked, StateFailedNonDestructive},
	StateLocked:                   {StatePrecheckPassed, StateFailedNonDestructive},
	StatePrecheckPassed:           {StateSourceQuiescing, StateFailedNonDestructive},
	StateSourceQuiescing:          {StateSourceDetached, StateFailedNonDestructive, StateFailedNeedsRollback, StateCancelling},
	StateSourceDetached:           {StateHostReconfiguring, StateFailedNeedsRollback, StateCompensating},
	StateHostReconfiguring:        {StateRebooting, StateTargetAttaching, StateFailedNeedsRollback, StateCompensating},
	StateRebooting:                {StateHostReachable, StateFailedManualRecovery},
	StateHostReachable:            {StateTargetAttaching, StateFailedManualRecovery, StateCompensating},
	StateTargetAttaching:          {StateVerifying, StateFailedNeedsRollback, StateCompensating},
	StateVerifying:                {StateCompleted, StateFailedNeedsRollback, StateCompensating},
	StateCompensating:             {StateCompleted, StateFailedNeedsRollback, StateFailedManualRecovery},
	StateCancelling:               {StateCancelled, StateFailedNonDestructive},
}

func IsValidTransition(from, to SwitchState) bool {
	targets, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}
