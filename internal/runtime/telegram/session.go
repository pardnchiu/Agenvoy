package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionHistory "github.com/pardnchiu/agenvoy/internal/session/history"
	"github.com/pardnchiu/agenvoy/internal/session/summary"
	sessionTelegram "github.com/pardnchiu/agenvoy/internal/session/telegram"
)

func getSession(ctx context.Context, chatID int64, username, content string, data exec.ExecData, overrideID, missingName string) (*agentTypes.AgentSession, error) {
	chatSessionID, err := sessionTelegram.New(chatID)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/agenvoy/internal/session GetTelegramSession: %w", err)
	}

	histSessionID := chatSessionID
	if id := strings.TrimSpace(overrideID); id != "" {
		histSessionID = id
	}

	sess := &agentTypes.AgentSession{
		ID:        histSessionID,
		Tools:     []agentTypes.Message{},
		Histories: []agentTypes.Message{},
	}

	oldHistory, maxHistory := sessionHistory.Get(histSessionID)
	sess.Histories = oldHistory
	sess.BaseLen = len(oldHistory)

	sess.SystemPrompts = exec.BuildSystemPrompts(data.WorkDir, data.ExtraSystemPrompt, agents.Scanner(), chatSessionID, data.AllowAll, data.ExcludeSkills)
	if summary := summary.GetPrompt(histSessionID, exec.OldestMessageTime(maxHistory)); summary != "" {
		sess.SummaryMessage = agentTypes.Message{Role: "assistant", Content: summary}
	}

	sess.OldHistories = maxHistory
	sess.ToolHistories = []agentTypes.Message{}

	header := fmt.Sprintf("當前時間: %s\n工作目錄: %s\n傳送者: %s\n當前 chat ID: %d",
		time.Now().Format("2006-01-02 15:04:05"),
		data.WorkDir,
		username,
		chatID,
	)
	if name := strings.TrimSpace(missingName); name != "" {
		header += fmt.Sprintf("\n備註: 找不到 session %q，改以當前 chat session 處理", name)
	}
	userText := fmt.Sprintf("---\n%s\n---\n%s", header, strings.TrimSpace(content))

	sess.Histories = append(sess.Histories, agentTypes.Message{
		Role:    "user",
		Content: userText,
	})
	sess.UserInput = agentTypes.Message{
		Role:    "user",
		Content: userText,
	}
	exec.SaveUserInputHistory(ctx, histSessionID, userText)

	return sess, nil
}
