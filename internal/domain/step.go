package domain

import "time"

type StepRecord struct {
	ExecutionID        string
	StepName           string
	Sequence           int
	Host               string
	StartedAt          time.Time
	EndedAt            *time.Time
	Status             StepStatus
	RetryCount         int
	ExitCode           *int
	ErrorClass         FailureClass
	CommandID          string
	StdoutPath         string
	StderrPath         string
	SnapshotBeforePath string
	SnapshotAfterPath  string
}
