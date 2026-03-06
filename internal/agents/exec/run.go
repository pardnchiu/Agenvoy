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
	matchedSkill := SelectSkill(ctx, bot, scanner, trimInput)
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

	agent := SelectAgent(ctx, bot, registry, trimInput)
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
	return Execute(ctx, execData, session, events, allowAll)
}
