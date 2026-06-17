package domain

import "time"

// AdminTokenRenewalTrigger identifies why an SSH-minted admin token was issued.
type AdminTokenRenewalTrigger string

const (
	// AdminTokenRenewalTriggerCacheMiss marks the initial mint when no admin
	// token was cached yet.
	AdminTokenRenewalTriggerCacheMiss AdminTokenRenewalTrigger = "cache_miss"
	// AdminTokenRenewalTriggerAuthFailure marks a remint triggered by an
	// admin-authentication failure from slurmrestd.
	AdminTokenRenewalTriggerAuthFailure AdminTokenRenewalTrigger = "auth_failure"
)

// AdminTokenRenewal is an audit record for one successful SSH-minted Slurm
// admin token issuance. It deliberately omits the minted JWT itself.
type AdminTokenRenewal struct {
	ID        int64
	IssuedAt  time.Time
	AdminUser string
	LoginNode string
	Trigger   AdminTokenRenewalTrigger
}
