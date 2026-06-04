package domain

import "time"

type Execution struct {
	ID                       string
	NodeName                 string
	Direction                SwitchDirection
	RequestedBy              string
	RequestedAt              time.Time
	CurrentState             SwitchState
	DesiredOwner             Owner
	PreviousOwner            Owner
	StateVersion             int64
	OverallStatus            OverallStatus
	LockAcquiredAt           *time.Time
	LockReleasedAt           *time.Time
	FinalErrorCode           string
	FinalErrorSummary        string
	LogRoot                  string
	PlaceholderJobID          string
	RequestedSlurmConstraint  string
	RequestedSlurmPartition   string
	AllocationEventAt         *time.Time
	CancellationSourceState   SwitchState
}
