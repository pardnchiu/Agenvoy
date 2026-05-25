package exec

import (
	"context"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/summary"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

func ForceSummary(ctx context.Context, sessionID string) (int, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("session id is required")
	}

	histories, _ := sessionManager.GetHistory(sessionID)
	summaryHistories := summary.Get(histories)
	if len(summaryHistories) == 0 {
		return 0, nil
	}

	agent := SelectAgent(ctx, agents.Dispatcher(), agents.Registry(), "[summary] force refresh", false, sessionID)
	if agent == nil {
		return 0, fmt.Errorf("no agent available for summary refresh")
	}
	if err := summary.Generate(ctx, agent, sessionID, summaryHistories); err != nil {
		return 0, fmt.Errorf("summary refresh failed: %w", err)
	}
	return len(summaryHistories), nil
}

func ResetSessionWithSummary(ctx context.Context, sessionID string) (int, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("session id is required")
	}

	histories, _ := sessionManager.GetHistory(sessionID)
	summaryHistories := summary.Get(histories)

	if len(summaryHistories) > 0 {
		agent := SelectAgent(ctx, agents.Dispatcher(), agents.Registry(), "[summary] reset session", false, sessionID)
		if agent == nil {
			return 0, fmt.Errorf("no agent available for summary refresh; reset aborted")
		}
		if err := summary.Generate(ctx, agent, sessionID, summaryHistories); err != nil {
			return 0, fmt.Errorf("summary refresh failed; reset aborted to avoid context loss: %w", err)
		}
	}

	return sessionManager.ResetHistoryKeepSummary(sessionID)
}
