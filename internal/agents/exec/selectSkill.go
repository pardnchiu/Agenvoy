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

// func SelectTools(ctx context.Context, bot agentTypes.Agent, userInput string, fileNames []string) []toolTypes.Tool {
// 	tools := toolRegister.JSON()
// 	if len(tools) == 0 {
// 		return nil
// 	}

// 	userContent := strings.TrimSpace(userInput)
// 	if len(fileNames) > 0 {
// 		userContent += fmt.Sprintf("\nAttached files: %s", strings.Join(fileNames, ", "))
// 	}

// 	systemprompt := `## System Context
// 你是一個工具選擇分析器。根據使用者需求，從工具列表中選出**所有可能用到的工具**。
// 回應必須是**純 JSON array**，禁止包含任何 Markdown 語法、程式碼區塊標記或額外說明文字。

// ## 工具列表
// ` + string(tools) + `

// ## 任務規則
// 1. 僅選擇需求**直接需要**的工具
// 2. 功能重疊時選最精確者
// 3. 輸出為純 JSON array，每個元素保留原始 name 與 description 欄位，不新增任何其他欄位

// ## 輸出範例
// [
//   {"name": "read_file", "description": "...原始描述..."},
//   {"name": "patch_edit", "description": "...原始描述..."}
// ]`

// 	messages := []agentTypes.Message{
// 		{
// 			Role:    "system",
// 			Content: systemprompt,
// 		},
// 		{
// 			Role:    "user",
// 			Content: userContent,
// 		},
// 	}

// 	resp, err := bot.Send(ctx, messages, nil)
// 	if err != nil || len(resp.Choices) == 0 {
// 		return nil
// 	}

// 	answer := ""
// 	if content, ok := resp.Choices[0].Message.Content.(string); ok {
// 		answer = strings.Trim(strings.TrimSpace(content), "\"'` \n")
// 	}

// 	if answer == "NONE" || answer == "" {
// 		return nil
// 	}

// 	slog.Info("SelectTools answer", "answer", answer)

// 	return nil
// }

func SelectSkill(ctx context.Context, bot agentTypes.Agent, scanner *skill.SkillScanner, userInput string, fileNames []string) *skill.Skill {
	skills := scanner.List()
	if len(skills) == 0 {
		return nil
	}

	const maxDescLen = 256

	skillMap := make(map[string]string, len(skills))
	for _, name := range skills {
		// * already checked List() will output trimmed skill name
		desc := strings.TrimSpace(scanner.Skills.ByName[name].Description)
		if len([]rune(desc)) > maxDescLen {
			desc = string([]rune(desc)[:maxDescLen-1]) + "…"
		}
		skillMap[name] = desc
	}
	skillJson, err := json.Marshal(skillMap)
	if err != nil {
		return nil
	}

	userContent := strings.TrimSpace(userInput)
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
