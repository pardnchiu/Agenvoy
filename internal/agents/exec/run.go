package exec

import (
	"context"
	"fmt"
	"os"
	"strings"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/skill"
)

func Run(ctx context.Context, bot agentTypes.Agent, registry agentTypes.AgentRegistry, scanner *skill.SkillScanner, userInput string, imageInputs []string, fileInputs []string, events chan<- agentTypes.Event, allowAll bool) error {
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("os.Getwd: %w", err)
	}

	trimInput := strings.TrimSpace(userInput)

	events <- agentTypes.Event{
		Type: agentTypes.EventSkillSelect,
	}
	fileNames := make([]string, len(fileInputs))
	for i, f := range fileInputs {
		fileNames[i] = f
	}
	matchedSkill := SelectSkill(ctx, bot, scanner, trimInput, fileNames)
	if matchedSkill != nil {
		events <- agentTypes.Event{
			Type: agentTypes.EventSkillResult,
			Text: strings.TrimSpace(matchedSkill.Name),
		}
	} else {
		events <- agentTypes.Event{
			Type: agentTypes.EventSkillResult,
			Text: "none",
		}
	}

	events <- agentTypes.Event{
		Type: agentTypes.EventAgentSelect,
	}

	// SelectTools(ctx, bot, trimInput, fileNames)

	agent := SelectAgent(ctx, bot, registry, trimInput, matchedSkill != nil)
	events <- agentTypes.Event{
		Type: agentTypes.EventAgentResult,
		Text: strings.TrimSpace(agent.Name()),
	}

	execData := ExecData{
		Agent:       agent,
		WorkDir:     workDir,
		Skill:       matchedSkill,
		Content:     trimInput,
		ImageInputs: imageInputs,
		FileInputs:  fileInputs,
	}
	session, err := GetSession(execData)
	if err != nil {
		return fmt.Errorf("GetSession: %w", err)
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
	events <- agentTypes.Event{Type: agentTypes.EventSummaryGenerate}
	GenerateSummary(context.Background(), SelectAgent(ctx, bot, registry, "[summary] 整理對話摘要，選擇最輕量可完成任務的模型", false), session.ID, session.Histories)
	if finalDone != nil {
		events <- *finalDone
	}
	return nil
}
