package handler

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pardnchiu/go-utils/utils"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/external"
	"github.com/pardnchiu/agenvoy/internal/agents/host"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
)

type Request struct {
	Content      string   `json:"content"`
	SSE          bool     `json:"sse"`
	SessionID    string   `json:"session_id"`
	Model        string   `json:"model,omitempty"`
	ExcludeTools []string `json:"exclude_tools,omitempty"`
	Persist      bool     `json:"persist,omitempty"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
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

		go sessionManager.Clean()

		sessionID := req.SessionID
		if sessionID == "" {
			prefix := "temp-"
			if req.Persist {
				prefix = "http-"
			}
			sessionID = prefix + utils.UUID()
		}

		events := make(chan agentTypes.Event, 64)
		ctx := c.Request.Context()

		go func() {
			defer close(events)

			trimContent := strings.TrimSpace(req.Content)

			externalAgent, externalEffective, externalReadOnly := external.MatchExternal(trimContent)
			if externalAgent != "" {
				trimContent = strings.TrimSpace(externalEffective)
			}

			var matchedSkill *skill.Skill
			if externalAgent == "" && scanner != nil {
				if m, effective := scanner.MatchSkillCall(trimContent); m != nil {
					matchedSkill = m
					trimContent = strings.TrimSpace(effective)
					events <- agentTypes.Event{Type: agentTypes.EventSkillResult, Text: strings.TrimSpace(m.Name)}
				}
			}

			events <- agentTypes.Event{Type: agentTypes.EventAgentSelect}
			var agent agentTypes.Agent
			if externalAgent != "" {
				events <- agentTypes.Event{Type: agentTypes.EventAgentResult, Text: "external:" + externalAgent}
			} else {
				if req.Model != "" {
					if a, ok := registry.Registry[req.Model]; ok {
						agent = a
					}
				}
				if agent == nil {
					agent = exec.SelectAgent(ctx, bot, registry, trimContent, false)
				}
				events <- agentTypes.Event{Type: agentTypes.EventAgentResult, Text: agent.Name()}
			}

			workDir, _ := os.UserHomeDir()
			data := exec.ExecData{
				Agent:             agent,
				WorkDir:           workDir,
				Skill:             matchedSkill,
				Content:           trimContent,
				ExcludeTools:      req.ExcludeTools,
				ExtraSystemPrompt: req.SystemPrompt,
			}

			sessionManager.SaveBot(sessionID, sessionID, false)

			session, err := newSession(data, sessionID)
			if err != nil {
				events <- agentTypes.Event{Type: agentTypes.EventError, Err: err}
				return
			}

			if externalAgent != "" {
				if err := exec.CallExternal(ctx, session.ID, externalAgent, trimContent, externalReadOnly, events); err != nil {
					events <- agentTypes.Event{Type: agentTypes.EventError, Err: err}
				}
				return
			}

			if err := exec.Execute(ctx, data, session, events, true); err != nil {
				events <- agentTypes.Event{Type: agentTypes.EventError, Err: err}
				return
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
		Histories: []agentTypes.Message{},
	}

	scanner := data.SkillScanner
	if scanner == nil {
		scanner = host.Scanner()
	}
	session.SystemPrompts = []agentTypes.Message{{Role: "system", Content: exec.GetSystemPrompt(data.WorkDir, data.ExtraSystemPrompt, scanner, sessionID)}}

	oldHistory, maxHistory := sessionManager.GetHistory(sessionID)
	session.Histories = oldHistory
	session.OldHistories = maxHistory

	if summary := sessionManager.GetSummaryPrompt(sessionID, exec.OldestMessageTime(maxHistory)); summary != "" {
		session.SummaryMessage = agentTypes.Message{Role: "assistant", Content: summary}
	}
	session.ToolHistories = []agentTypes.Message{}

	userText := fmt.Sprintf("---\n當前時間: %s\n---\n%s",
		time.Now().Format("2006-01-02 15:04:05"), data.Content)
	session.UserInput = agentTypes.Message{Role: "user", Content: userText}
	session.Histories = append(session.Histories, agentTypes.Message{
		Role:    "user",
		Content: userText,
	})
	exec.SaveUserInputHistory(sessionID, userText)

	return session, nil
}
