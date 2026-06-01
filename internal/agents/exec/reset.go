package exec

import (
	"context"
	"fmt"

	"github.com/pardnchiu/agenvoy/internal/agents"
	agentSummary "github.com/pardnchiu/agenvoy/internal/agents/exec/summary"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	sessionHistory "github.com/pardnchiu/agenvoy/internal/session/history"
)

func summaryRouter() agentTypes.Agent {
	if a := agents.SummaryBot(); a != nil {
		return a
	}
	return agents.DispatcherBot()
}

func ForceSummary(ctx context.Context, sessionID string) (int, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("session id is required")
	}

	_, histories := sessionHistory.Get(sessionID)
	if len(histories) == 0 {
		return 0, nil
	}

	agent := SelectAgent(ctx, summaryRouter(), agents.Registry(), "[summary] force refresh", false, sessionID)
	if agent == nil {
		return 0, fmt.Errorf("no agent available for summary refresh")
	}
	if err := agentSummary.Generate(ctx, agent, sessionID, histories); err != nil {
		return 0, fmt.Errorf("summary refresh failed: %w", err)
	}
	return len(histories), nil
}

func ResetSessionWithSummary(ctx context.Context, sessionID string) (int, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("session id is required")
	}

	_, histories := sessionHistory.Get(sessionID)

	if len(histories) > 0 {
		agent := SelectAgent(ctx, summaryRouter(), agents.Registry(), "[summary] reset session", false, sessionID)
		if agent == nil {
			return 0, fmt.Errorf("no agent available for summary refresh; reset aborted")
		}
		if err := agentSummary.Generate(ctx, agent, sessionID, histories); err != nil {
			return 0, fmt.Errorf("summary refresh failed; reset aborted to avoid context loss: %w", err)
		}
	}

	return sessionManager.Reset(sessionID)
}
