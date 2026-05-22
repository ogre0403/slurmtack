package slurm

import (
	"fmt"
	"strings"
)

type SlurmAPIError struct {
	StatusCode int
	Messages   []string
}

func (e *SlurmAPIError) Error() string {
	return fmt.Sprintf("slurmrestd %d: %s", e.StatusCode, strings.Join(e.Messages, "; "))
}
