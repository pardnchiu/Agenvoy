package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

func getSession(chatID int64, username, content string, data exec.ExecData) (*agentTypes.AgentSession, error) {
	sessionID, err := sessionManager.GetTelegramSession(chatID)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/agenvoy/internal/session GetTelegramSession: %w", err)
	}

	sess := &agentTypes.AgentSession{
		ID:        sessionID,
		Tools:     []agentTypes.Message{},
		Histories: []agentTypes.Message{},
	}

	oldHistory, maxHistory := sessionManager.GetHistory(sessionID)
	sess.Histories = oldHistory

	sess.SystemPrompts = exec.BuildSystemPrompts(data.WorkDir, data.ExtraSystemPrompt, host.Scanner(), sessionID, data.AllowAll, false)
	if summary := sessionManager.GetSummaryPrompt(sessionID, exec.OldestMessageTime(maxHistory)); summary != "" {
		sess.SummaryMessage = agentTypes.Message{Role: "assistant", Content: summary}
	}

	sess.OldHistories = maxHistory
	sess.ToolHistories = []agentTypes.Message{}

	userText := fmt.Sprintf("---\n當前時間: %s\n工作目錄: %s\n傳送者: %s\n當前 chat ID: %d\n---\n%s",
		time.Now().Format("2006-01-02 15:04:05"),
		data.WorkDir,
		username,
		chatID,
		strings.TrimSpace(content),
	)

	sess.Histories = append(sess.Histories, agentTypes.Message{
		Role:    "user",
		Content: userText,
	})
	sess.UserInput = agentTypes.Message{
		Role:    "user",
		Content: userText,
	}
	exec.SaveUserInputHistory(sessionID, userText)

	return sess, nil
}
