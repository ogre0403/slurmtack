package domain

import "testing"

func TestValidTransitions(t *testing.T) {
	cases := []struct {
		from  SwitchState
		to    SwitchState
		valid bool
	}{
		{StateRequested, StateAwaitingSourceAllocation, true},
		{StateRequested, StateAwaitingTargetNode, true},
		{StateRequested, StateNodeIdentified, true},
		{StateRequested, StateLocked, true},
		{StateRequested, StateFailedNonDestructive, true},
		{StateRequested, StateCompleted, false},
		{StateAwaitingSourceAllocation, StateNodeIdentified, true},
		{StateAwaitingSourceAllocation, StateCompleted, false},
		{StateAwaitingTargetNode, StateNodeIdentified, true},
		{StateAwaitingTargetNode, StateLocked, false},
		{StateNodeIdentified, StateLocked, true},
		{StateNodeIdentified, StateSourceDetached, false},
		{StateLocked, StatePrecheckPassed, true},
		{StatePrecheckPassed, StateSourceQuiescing, true},
		{StateSourceQuiescing, StateSourceDetached, true},
		{StateSourceDetached, StateHostReconfiguring, true},
		{StateHostReconfiguring, StateRebooting, true},
		{StateHostReconfiguring, StateTargetAttaching, true},
		{StateRebooting, StateHostReachable, true},
		{StateRebooting, StateFailedManualRecovery, true},
		{StateRebooting, StateFailedNonDestructive, false},
		{StateHostReachable, StateTargetAttaching, true},
		{StateTargetAttaching, StateVerifying, true},
		{StateVerifying, StateCompleted, true},
		{StateCompleted, StateRequested, false},
	}

	for _, tc := range cases {
		got := IsValidTransition(tc.from, tc.to)
		if got != tc.valid {
			t.Errorf("IsValidTransition(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.valid)
		}
	}
}

func TestTerminalStates(t *testing.T) {
	terminals := []SwitchState{
		StateCompleted,
		StateFailedNonDestructive,
		StateFailedNeedsRollback,
		StateFailedManualRecovery,
	}
	for _, s := range terminals {
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", s)
		}
	}

	nonTerminals := []SwitchState{
		StateRequested,
		StateAwaitingSourceAllocation,
		StateAwaitingTargetNode,
		StateLocked,
		StateRebooting,
		StateCompensating,
	}
	for _, s := range nonTerminals {
		if s.IsTerminal() {
			t.Errorf("%s should not be terminal", s)
		}
	}
}
