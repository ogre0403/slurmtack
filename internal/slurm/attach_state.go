package slurm

import (
	"context"
	"fmt"
	"strings"
)

type AttachStateDisposition int

const (
	AttachStateResumeRequired AttachStateDisposition = iota
	AttachStateReady
	AttachStateUnsupported
)

var resumableAttachStateTokens = map[string]struct{}{
	"drain":   {},
	"drained": {},
	"down":    {},
}

var activeAttachStateTokens = map[string]struct{}{
	"idle":  {},
	"alloc": {},
	"mixed": {},
}

func ClassifyAttachState(state string) AttachStateDisposition {
	sawActive := false
	for _, token := range strings.Split(strings.ToLower(strings.TrimSpace(state)), "+") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if _, ok := resumableAttachStateTokens[token]; ok {
			return AttachStateResumeRequired
		}
		if _, ok := activeAttachStateTokens[token]; ok {
			sawActive = true
			continue
		}
		return AttachStateUnsupported
	}
	if sawActive {
		return AttachStateReady
	}
	return AttachStateUnsupported
}

func EnsureNodeReadyForAttach(ctx context.Context, client Client, nodeName string) error {
	state, err := client.GetNodeState(ctx, nodeName)
	if err != nil {
		return fmt.Errorf("getting node state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("node %s state missing", nodeName)
	}

	switch ClassifyAttachState(state.State) {
	case AttachStateResumeRequired:
		return client.ResumeNode(ctx, nodeName)
	case AttachStateReady:
		return nil
	default:
		return fmt.Errorf("node %s not attachable (state: %s)", nodeName, state.State)
	}
}
