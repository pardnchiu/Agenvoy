package chatCompletions

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/external"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
)

func run(ctx context.Context, req Request, sessionID, userContent string, events chan<- agentTypes.Event) {
	scanner := agents.Scanner()
	if scanner != nil {
		scanner.Scan()
	}

	trimContent := strings.TrimSpace(userContent)
	if trimContent != "" {
		events <- agentTypes.Event{Type: agentTypes.EventUserInput, Text: trimContent}
	}

	externalAgent, externalEffective, externalReadOnly := external.MatchExternal(trimContent)
	if externalAgent != "" {
		trimContent = strings.TrimSpace(externalEffective)
	}

	var matchedSkill *filesystem.Skill
	var skillResult agentTypes.Event
	if externalAgent == "" && scanner != nil {
		if m, effective := runtime.MatchSkill(scanner, trimContent); m != nil {
			matchedSkill = m
			trimContent = strings.TrimSpace(effective)
			skillResult = agentTypes.Event{Type: agentTypes.EventSkillResult, Text: strings.TrimSpace(m.Name)}
			events <- skillResult
			sessionManager.Record(sessionID, skillResult)
		}
	}

	events <- agentTypes.Event{Type: agentTypes.EventAgentSelect}

	var agent agentTypes.Agent
	var fallbacks []agentTypes.Agent
	var agentResult agentTypes.Event
	registry := agents.Registry()
	if externalAgent != "" {
		agentResult = agentTypes.Event{Type: agentTypes.EventAgentResult, Text: "external:" + externalAgent}
	} else {
		switch {
		case req.Model == "" || req.Model == "auto":
			primary, rest, err := exec.ResolveAgent(ctx, agents.Dispatcher(), registry, trimContent, matchedSkill != nil, sessionID)
			if err != nil {
				events <- agentTypes.Event{Type: agentTypes.EventError, Err: err}
				return
			}
			agent = primary
			fallbacks = rest

		default:
			a, ok := registry.Registry[req.Model]
			if !ok {
				events <- agentTypes.Event{Type: agentTypes.EventError, Err: fmt.Errorf("model %q not found", req.Model)}
				return
			}
			agent = a
		}
		agentResult = agentTypes.Event{Type: agentTypes.EventAgentResult, Text: strings.TrimSpace(agent.Name())}
	}
	events <- agentResult
	sessionManager.Record(sessionID, agentResult)

	workDir := req.workDir
	if workDir == "" {
		workDir, _ = os.UserHomeDir()
	}
	data := exec.ExecData{
		Agent:          agent,
		FallbackAgents: fallbacks,
		WorkDir:        workDir,
		Skill:          matchedSkill,
		Content:        trimContent,
		SessionID:      sessionID,
		AllowAll:       true,
	}

	sessionManager.SaveBot(sessionID, sessionID, false)

	session, err := exec.GetSession(data)
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
	}
}
