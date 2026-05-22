package orchestrator

import "errors"

var ErrSSHPollTimeout = errors.New("ssh poll timeout: host did not become reachable")
