package line

import (
	"fmt"
	"strings"
	"time"

	go_bot_line "github.com/pardnchiu/go-bot/line"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	sessionHistory "github.com/pardnchiu/agenvoy/internal/session/history"
	"github.com/pardnchiu/agenvoy/internal/session/summary"
)

func getSession(in go_bot_line.Input, content string, data exec.ExecData) (*agentTypes.AgentSession, error) {
	sessionID, err := sessionManager.GetLineSession(in.SourceType, in.UserID, in.GroupID, in.RoomID)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/agenvoy/internal/session GetLineSession: %w", err)
	}

	sess := &agentTypes.AgentSession{
		ID:        sessionID,
		Tools:     []agentTypes.Message{},
		Histories: []agentTypes.Message{},
	}

	oldHistory, maxHistory := sessionHistory.Get(sessionID)
	sess.Histories = oldHistory
	sess.BaseLen = len(oldHistory)

	sess.SystemPrompts = exec.BuildSystemPrompts(data.WorkDir, data.ExtraSystemPrompt, agents.Scanner(), sessionID, data.AllowAll, false, data.ExcludeSkills)
	if summary := summary.GetPrompt(sessionID, exec.OldestMessageTime(maxHistory)); summary != "" {
		sess.SummaryMessage = agentTypes.Message{Role: "assistant", Content: summary}
	}

	sess.OldHistories = maxHistory
	sess.ToolHistories = []agentTypes.Message{}

	header := fmt.Sprintf("當前時間: %s\n工作目錄: %s\n傳送者: %s\n來源: LINE %s",
		time.Now().Format("2006-01-02 15:04:05"),
		data.WorkDir,
		sourceName(in),
		in.SourceType,
	)
	userText := fmt.Sprintf("---\n%s\n---\n%s", header, strings.TrimSpace(content))

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
