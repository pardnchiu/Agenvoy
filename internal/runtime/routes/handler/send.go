package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pardnchiu/go-pkg/utils"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/external"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	"github.com/pardnchiu/agenvoy/internal/runtime/pubsub"
	configBot "github.com/pardnchiu/agenvoy/internal/session/config/bot"
	sessionHistory "github.com/pardnchiu/agenvoy/internal/session/history"
	sessionLog "github.com/pardnchiu/agenvoy/internal/session/log"
	"github.com/pardnchiu/agenvoy/internal/session/summary"
	"github.com/pardnchiu/agenvoy/internal/tools"
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

func Send() gin.HandlerFunc {
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
			prefix := "temp-"
			if req.Persist {
				prefix = "http-"
			}
			sessionID = prefix + utils.UUID()
		}

		events := make(chan agentTypes.Event, 64)
		execCtx := context.WithoutCancel(c.Request.Context())
		wrapped := pubsub.Wrap(execCtx, sessionID, events, 64)

		go func() {
			defer close(wrapped)

			scanner := agents.Scanner()
			if scanner != nil {
				scanner.Scan()
			}
			trimContent := strings.TrimSpace(req.Content)
			if trimContent != "" {
				wrapped <- agentTypes.Event{Type: agentTypes.EventUserInput, Text: trimContent}
			}

			externalAgent, externalEffective, externalReadOnly := external.MatchExternal(trimContent)
			if externalAgent != "" {
				trimContent = strings.TrimSpace(externalEffective)
			}

			var matchedSkill *skill.Skill
			var skillResult agentTypes.Event
			if externalAgent == "" && scanner != nil {
				if m, effective := runtime.MatchSkill(scanner, trimContent, tools.TUIOnlySkills...); m != nil {
					matchedSkill = m
					trimContent = strings.TrimSpace(effective)
					skillResult = agentTypes.Event{Type: agentTypes.EventSkillResult, Text: strings.TrimSpace(m.Name)}
					wrapped <- skillResult
					if sessionID != "" {
						sessionLog.Record(sessionID, skillResult)
					}
				}
			}

			wrapped <- agentTypes.Event{Type: agentTypes.EventAgentSelect}
			var agent agentTypes.Agent
			var fallbacks []agentTypes.Agent
			var agentResult agentTypes.Event
			if externalAgent != "" {
				agentResult = agentTypes.Event{Type: agentTypes.EventAgentResult, Text: "external:" + externalAgent}
			} else {
				registry := agents.Registry()
				if req.Model != "" {
					if a, ok := registry.Registry[req.Model]; ok {
						agent = a
					}
				}
				if agent == nil {
					primary, rest, err := exec.ResolveAgent(execCtx, agents.DispatcherBot(), registry, trimContent, false, sessionID)
					if err != nil {
						wrapped <- agentTypes.Event{Type: agentTypes.EventError, Err: err}
						return
					}
					agent = primary
					fallbacks = rest
				}
				agentResult = agentTypes.Event{Type: agentTypes.EventAgentResult, Text: agent.Name()}
			}
			wrapped <- agentResult
			if sessionID != "" {
				sessionLog.Record(sessionID, agentResult)
			}

			workDir, _ := os.UserHomeDir()
			data := exec.ExecData{
				Agent:             agent,
				FallbackAgents:    fallbacks,
				WorkDir:           workDir,
				Skill:             matchedSkill,
				Content:           trimContent,
				ExcludeTools:      append(append([]string{}, tools.TUIOnlyTools...), req.ExcludeTools...),
				ExcludeSkills:     tools.TUIOnlySkills,
				ExtraSystemPrompt: req.SystemPrompt,
				AllowAll:          true,
			}

			if err := configBot.Save(sessionID, "", "", false); err != nil {
				slog.Warn("sessionBot Save",
					slog.String("session", sessionID),
					slog.String("error", err.Error()))
			}

			session, err := newSession(execCtx, data, sessionID)
			if err != nil {
				wrapped <- agentTypes.Event{Type: agentTypes.EventError, Err: err}
				return
			}

			if externalAgent != "" {
				if err := exec.CallExternal(execCtx, session.ID, externalAgent, trimContent, externalReadOnly, wrapped); err != nil {
					wrapped <- agentTypes.Event{Type: agentTypes.EventError, Err: err}
				}
				return
			}

			if err := exec.Execute(execCtx, data, session, wrapped, true); err != nil {
				wrapped <- agentTypes.Event{Type: agentTypes.EventError, Err: err}
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

func newSession(ctx context.Context, data exec.ExecData, sessionID string) (*agentTypes.AgentSession, error) {
	session := &agentTypes.AgentSession{
		ID:        sessionID,
		Tools:     []agentTypes.Message{},
		Histories: []agentTypes.Message{},
	}

	scanner := data.SkillScanner
	if scanner == nil {
		scanner = agents.Scanner()
	}
	session.SystemPrompts = exec.BuildSystemPrompts(data.WorkDir, data.ExtraSystemPrompt, scanner, sessionID, data.AllowAll, data.ExcludeSkills)

	oldHistory, maxHistory := sessionHistory.Get(sessionID)
	session.Histories = oldHistory
	session.BaseLen = len(oldHistory)
	session.OldHistories = maxHistory

	if summary := summary.GetPrompt(sessionID, exec.OldestMessageTime(maxHistory)); summary != "" {
		session.SummaryMessage = agentTypes.Message{Role: "assistant", Content: summary}
	}
	session.ToolHistories = []agentTypes.Message{}

	userText := fmt.Sprintf("---\n當前時間: %s\n工作目錄: %s\n---\n%s",
		time.Now().Format("2006-01-02 15:04:05"), data.WorkDir, data.Content)
	session.UserInput = agentTypes.Message{Role: "user", Content: userText}
	session.Histories = append(session.Histories, agentTypes.Message{
		Role:    "user",
		Content: userText,
	})
	exec.SaveUserInputHistory(ctx, sessionID, userText)

	return session, nil
}
