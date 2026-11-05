package exec

import (
	"context"
	"errors"
	"strings"

	go_llm_router "github.com/pardnchiu/go-llm-router/core"
)

func isSendTimeoutError(err error, sendCtxErr error) bool {
	if errors.Is(sendCtxErr, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	s := err.Error()

	if strings.Contains(s, "Client.Timeout") ||
		strings.Contains(s, "context deadline exceeded") ||
		strings.Contains(s, "timeout awaiting response headers") ||
		strings.Contains(s, "TLS handshake timeout") ||
		strings.Contains(s, "i/o timeout") ||
		strings.Contains(s, "degenerate repetition") {
		return true
	}
	return false
}

func isReasoningRejected(err error) bool {
	s := strings.ToLower(err.Error())
	if !strings.Contains(s, "thinking") && !strings.Contains(s, "reasoning") && !strings.Contains(s, "effort") {
		return false
	}
	return strings.Contains(s, "not supported") ||
		strings.Contains(s, "unsupported") ||
		strings.Contains(s, "does not support") ||
		strings.Contains(s, "not allowed") ||
		strings.Contains(s, "invalid value") ||
		strings.Contains(s, "instead of")
}

func stepReasoning(r go_llm_router.Reasoning, dir int) (go_llm_router.Reasoning, int, bool) {
	if dir == 0 {
		dir = -1
		if r < go_llm_router.ReasoningMedium {
			dir = 1
		}
	}
	next := r + go_llm_router.Reasoning(dir)
	if next < go_llm_router.ReasoningNone || next > go_llm_router.ReasoningMax {
		return r, dir, false
	}
	return next, dir, true
}
