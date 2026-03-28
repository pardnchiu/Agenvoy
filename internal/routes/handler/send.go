package handler

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

type Request struct {
	Content   string `json:"content"`
	SSE       bool   `json:"sse"`
	SessionID string `json:"session_id"`
}

func Send(bot agentTypes.Agent, registry agentTypes.AgentRegistry, scanner *skill.SkillScanner) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req Request
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if strings.TrimSpace(req.Content) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
			return
		}

		sessionID := req.SessionID
		if sessionID == "" {
			sessionID = "http-" + utils.UUID()
		}

		events := make(chan agentTypes.Event, 64)
		ctx := c.Request.Context()

		go func() {
			defer close(events)

			trimContent := strings.TrimSpace(req.Content)

			events <- agentTypes.Event{Type: agentTypes.EventSkillSelect}
			skill := exec.SelectSkill(ctx, bot, scanner, trimContent, nil)
			if skill != nil {
				events <- agentTypes.Event{Type: agentTypes.EventSkillResult, Text: skill.Name}
			} else {
				events <- agentTypes.Event{Type: agentTypes.EventSkillResult, Text: "none"}
			}

			events <- agentTypes.Event{Type: agentTypes.EventAgentSelect}
			agent := exec.SelectAgent(ctx, bot, registry, trimContent, skill != nil)
			events <- agentTypes.Event{Type: agentTypes.EventAgentResult, Text: agent.Name()}

			workDir, _ := os.UserHomeDir()
			data := exec.ExecData{
				Agent:   agent,
				WorkDir: workDir,
				Skill:   skill,
				Content: trimContent,
			}

			session, err := newSession(data, sessionID)
			if err != nil {
				events <- agentTypes.Event{Type: agentTypes.EventError, Err: err}
				return
			}

			if err := exec.Execute(ctx, data, session, events, true); err != nil {
				events <- agentTypes.Event{Type: agentTypes.EventError, Err: err}
			}
		}()

		if req.SSE {
			sendSSE(c, sessionID, req.Content, events)
		} else {
			sendResult(c, sessionID, req.Content, events)
		}
	}
}

func newSession(data exec.ExecData, sessionID string) (*agentTypes.AgentSession, error) {
	session := &agentTypes.AgentSession{
		ID:        sessionID,
		Tools:     []agentTypes.Message{},
		Messages:  []agentTypes.Message{},
		Histories: []agentTypes.Message{},
	}

	session.Messages = append(session.Messages, agentTypes.Message{
		Role:    "system",
		Content: exec.GetSystemPrompt(data),
	})

	if summary := sessionManager.GetSummaryPrompt(sessionID); summary != "" {
		session.Messages = append(session.Messages, agentTypes.Message{
			Role:    "system",
			Content: summary,
		})
	}

	oldHistory, maxHistory := sessionManager.GetHistory(sessionID)
	session.Histories = oldHistory
	session.Messages = append(session.Messages, maxHistory...)

	userText := fmt.Sprintf("---\n當前時間: %s\n---\n%s",
		time.Now().Format("2006-01-02 15:04:05"), data.Content)
	session.Messages = append(session.Messages, agentTypes.Message{
		Role:    "user",
		Content: userText,
	})
	session.Histories = append(session.Histories, agentTypes.Message{
		Role:    "user",
		Content: userText,
	})

	return session, nil
}
