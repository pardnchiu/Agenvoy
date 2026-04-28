package exec

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/external"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionManager "github.com/pardnchiu/agenvoy/internal/session"
	"github.com/pardnchiu/agenvoy/internal/skill"
)

func Run(ctx context.Context, bot agentTypes.Agent, registry agentTypes.AgentRegistry, scanner *skill.SkillScanner, userInput string, imageInputs []string, fileInputs []string, events chan<- agentTypes.Event, allowAll bool) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("os.Getwd: %w", err)
	}

	trimInput := strings.TrimSpace(userInput)

	var sessionOverride string
	if name, effective := sessionManager.Match(trimInput); name != "" {
		resolved := sessionManager.GetSessionIDByName(name)
		if resolved == "" {
			return fmt.Errorf("session %q not found", name)
		}
		sessionOverride = resolved
		trimInput = strings.TrimSpace(effective)
	}

	externalAgent, externalEffective, externalReadOnly := external.MatchExternal(trimInput)
	if externalAgent != "" {
		trimInput = strings.TrimSpace(externalEffective)
	}

	var matchedSkill *skill.Skill
	if externalAgent == "" && scanner != nil {
		if m, effective := scanner.MatchSkillCall(trimInput); m != nil {
			matchedSkill = m
			trimInput = strings.TrimSpace(effective)
			events <- agentTypes.Event{Type: agentTypes.EventSkillResult, Text: strings.TrimSpace(m.Name)}
		}
	}

	events <- agentTypes.Event{
		Type: agentTypes.EventAgentSelect,
	}

	var agent agentTypes.Agent
	if externalAgent != "" {
		events <- agentTypes.Event{
			Type: agentTypes.EventAgentResult,
			Text: "external:" + externalAgent,
		}
	} else {
		agent = SelectAgent(ctx, bot, registry, trimInput, matchedSkill != nil)
		events <- agentTypes.Event{
			Type: agentTypes.EventAgentResult,
			Text: strings.TrimSpace(agent.Name()),
		}
	}

	execData := ExecData{
		Agent:       agent,
		WorkDir:     workDir,
		Skill:       matchedSkill,
		Content:     trimInput,
		SessionID:   sessionOverride,
		ImageInputs: imageInputs,
		FileInputs:  fileInputs,
		AllowAll:    allowAll,
	}
	session, err := GetSession(execData)
	if err != nil {
		return fmt.Errorf("GetSession: %w", err)
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
