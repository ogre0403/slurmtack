package slurm

import (
	"fmt"
	"net/http"
	"strings"
)

type SlurmAPIError struct {
	StatusCode int
	Messages   []string
}

func (e *SlurmAPIError) Error() string {
	return fmt.Sprintf("slurmrestd %d: %s", e.StatusCode, strings.Join(e.Messages, "; "))
}

func IsJobNotFound(err error) bool {
	if apiErr, ok := err.(*SlurmAPIError); ok {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// isAuthFailure reports whether err indicates the admin token was rejected as
// invalid or expired, which is the only condition that triggers token renewal.
func isAuthFailure(err error) bool {
	apiErr, ok := err.(*SlurmAPIError)
	if !ok {
		return false
	}
	if apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden {
		return true
	}
	for _, msg := range apiErr.Messages {
		l := strings.ToLower(msg)
		if strings.Contains(l, "invalid token") ||
			strings.Contains(l, "expired token") ||
			strings.Contains(l, "token expired") ||
			strings.Contains(l, "authentication") ||
			strings.Contains(l, "unauthorized") {
			return true
		}
	}
	return false
}
