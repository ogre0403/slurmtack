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
