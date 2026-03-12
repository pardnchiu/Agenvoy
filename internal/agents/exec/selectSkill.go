package exec

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/skill"
)

func SelectSkill(ctx context.Context, bot agentTypes.Agent, scanner *skill.SkillScanner, userInput string, fileNames []string) *skill.Skill {
	trimInput := strings.TrimSpace(userInput)

	skills := scanner.List()
	if len(skills) == 0 {
		return nil
	}

	skillMap := make(map[string]string, len(skills))
	for _, name := range skills {
		// * already checked List() will output trimmed skill name
		skillMap[name] = strings.TrimSpace(scanner.Skills.ByName[name].Description)
	}
	skillJson, err := json.Marshal(skillMap)
	if err != nil {
		return nil
	}

	userContent := strings.TrimSpace(trimInput)
	if len(fileNames) > 0 {
		userContent += fmt.Sprintf("\nAttached files: %s", strings.Join(fileNames, ", "))
	}

	messages := []agentTypes.Message{
		{
			Role:    "system",
			Content: strings.TrimSpace(configs.SkillSelector),
		},
		{
			Role: "user",
			Content: fmt.Sprintf(
				"Available skills: %s\nUser request: %s",
				string(skillJson),
				userContent,
			),
		},
	}

	resp, err := bot.Send(ctx, messages, nil)
	if err != nil || len(resp.Choices) == 0 {
		return nil
	}

	answer := ""
	if content, ok := resp.Choices[0].Message.Content.(string); ok {
		answer = strings.Trim(strings.TrimSpace(content), "\"'` \n")
	}

	if answer == "NONE" || answer == "" {
		return nil
	} else if s, ok := scanner.Skills.ByName[answer]; ok {
		return s
	}

	return nil
}
