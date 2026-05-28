package exec

import (
	"context"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/summary"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

func summaryRouter() agentTypes.Agent {
	if a := agents.Summary(); a != nil {
		return a
	}
	return agents.Dispatcher()
}

func ForceSummary(ctx context.Context, sessionID string) (int, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("session id is required")
	}

	histories, _ := sessionManager.GetHistory(sessionID)
	summaryHistories := summary.Get(histories)
	if len(summaryHistories) == 0 {
		return 0, nil
	}

	agent := SelectAgent(ctx, summaryRouter(), agents.Registry(), "[summary] force refresh", false, sessionID)
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
		agent := SelectAgent(ctx, summaryRouter(), agents.Registry(), "[summary] reset session", false, sessionID)
		if agent == nil {
			return 0, fmt.Errorf("no agent available for summary refresh; reset aborted")
		}
		if err := summary.Generate(ctx, agent, sessionID, summaryHistories); err != nil {
			return 0, fmt.Errorf("summary refresh failed; reset aborted to avoid context loss: %w", err)
		}
	}

	return sessionManager.ResetHistoryKeepSummary(sessionID)
}
