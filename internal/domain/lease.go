package domain

import "time"

type NodeLease struct {
	NodeName     string
	ExecutionID  string
	Holder       string
	ExpiresAt    time.Time
	StateVersion int64
}
