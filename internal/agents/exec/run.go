package exec

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/external"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	sessionLog "github.com/pardnchiu/agenvoy/internal/session/log"
)

func Run(ctx context.Context, bot agentTypes.Agent, registry agentTypes.AgentRegistry, scanner *runtime.SkillScanner, userInput string, imageInputs []string, fileInputs []string, events chan<- agentTypes.Event, allowAll bool, workDir, sessionID string, webMode bool) error {
	if strings.TrimSpace(workDir) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("os.Getwd: %w", err)
		}
		workDir = wd
	}
	sessionID = strings.TrimSpace(sessionID)

	trimInput := strings.TrimSpace(userInput)

	if trimInput != "" {
		events <- agentTypes.Event{Type: agentTypes.EventUserInput, Text: trimInput}
	}

	sessionOverride := sessionID
	if name, effective := sessionManager.Match(trimInput); name != "" {
		id := sessionManager.GetSessionID(name)
		if id == "" {
			return fmt.Errorf("session %q not found", name)
		}
		sessionOverride = id
		trimInput = strings.TrimSpace(effective)
	}

	externalAgent, externalEffective, externalReadOnly := external.MatchExternal(trimInput)
	if externalAgent != "" {
		trimInput = strings.TrimSpace(externalEffective)
	}

	var matchedSkill *filesystem.Skill
	var skillResult agentTypes.Event
	if externalAgent == "" && scanner != nil {
		if m, effective := runtime.MatchSkill(scanner, trimInput); m != nil {
			matchedSkill = m
			trimInput = strings.TrimSpace(effective)
			skillResult = agentTypes.Event{Type: agentTypes.EventSkillResult, Text: strings.TrimSpace(m.Name)}
			events <- skillResult
		}
	}

	events <- agentTypes.Event{
		Type: agentTypes.EventAgentSelect,
	}

	var agent agentTypes.Agent
	var fallbacks []agentTypes.Agent
	var agentResult agentTypes.Event
	if externalAgent != "" {
		agentResult = agentTypes.Event{
			Type: agentTypes.EventAgentResult,
			Text: "external:" + externalAgent,
		}
	} else {
		primary, rest, err := ResolveAgent(ctx, bot, registry, trimInput, matchedSkill != nil, sessionOverride)
		if err != nil {
			return fmt.Errorf("ResolveAgent: %w", err)
		}
		agent = primary
		fallbacks = rest
		agentResult = agentTypes.Event{
			Type: agentTypes.EventAgentResult,
			Text: strings.TrimSpace(agent.Name()),
		}
	}
	events <- agentResult

	execData := ExecData{
		Agent:          agent,
		FallbackAgents: fallbacks,
		WorkDir:        workDir,
		Skill:          matchedSkill,
		Content:        trimInput,
		SessionID:      sessionOverride,
		ImageInputs:    imageInputs,
		FileInputs:     fileInputs,
		AllowAll:       allowAll,
		WebMode:        webMode,
	}
	session, err := GetSession(execData)
	if err != nil {
		return fmt.Errorf("GetSession: %w", err)
	}

	if session != nil && session.ID != "" {
		if matchedSkill != nil {
			sessionLog.Record(session.ID, skillResult)
		}
		sessionLog.Record(session.ID, agentResult)
	}

	if externalAgent != "" {
		return CallExternal(ctx, session.ID, externalAgent, trimInput, externalReadOnly, events)
	}

	doneEvents := make(chan agentTypes.Event, 4)
	forwardEvents := make(chan agentTypes.Event, 16)
	execErrCh := make(chan error, 1)

	go func() {
		defer close(forwardEvents)
		for event := range doneEvents {
			if event.Type == agentTypes.EventDone {
				forwardEvents <- event
				continue
			}
			events <- event
		}
	}()

	go func() {
		execErrCh <- Execute(ctx, execData, session, doneEvents, allowAll)
		close(doneEvents)
	}()

	var finalDone *agentTypes.Event
	for event := range forwardEvents {
		if event.Type == agentTypes.EventDone {
			ev := event
			finalDone = &ev
			continue
		}
	}

	if err := <-execErrCh; err != nil {
		return err
	}
	if finalDone != nil {
		events <- *finalDone
	}
	return nil
}
