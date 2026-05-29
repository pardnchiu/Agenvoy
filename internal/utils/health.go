package utils

import (
	"context"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
)

func CheckHealth(ctx context.Context, a agentTypes.Agent, timeout time.Duration) bool {
	if a == nil {
		return false
	}
	hctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := a.Send(hctx, []agentTypes.Message{
		{Role: "system", Content: "Reply with only: ok"},
		{Role: "user", Content: "ping"},
	}, nil)
	if err != nil || resp == nil || len(resp.Choices) == 0 {
		return false
	}
	content, _ := resp.Choices[0].Message.Content.(string)
	return strings.TrimSpace(content) != ""
}
