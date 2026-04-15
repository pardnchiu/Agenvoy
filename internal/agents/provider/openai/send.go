package openai

import (
	"context"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	copilotResponse "github.com/pardnchiu/agenvoy/internal/agents/provider/copilot/response"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/skill"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	go_utils_http "github.com/pardnchiu/go-utils/http"
)

const (
	chatAPI      = "https://api.openai.com/v1/chat/completions"
	responsesAPI = "https://api.openai.com/v1/responses"
)

func (a *Agent) Execute(ctx context.Context, skill *skill.Skill, userInput string, events chan<- agentTypes.Event, allowAll bool) error {
	data := exec.ExecData{
		Agent:   a,
		WorkDir: a.workDir,
		Skill:   skill,
		Content: userInput,
	}
	session, err := exec.GetSession(data)
	if err != nil {
		return fmt.Errorf("exec.GetSession: %w", err)
	}
	return exec.Execute(ctx, data, session, events, allowAll)
}

func (a *Agent) Send(ctx context.Context, messages []agentTypes.Message, tools []toolTypes.Tool) (*agentTypes.Output, error) {
	headers := map[string]string{
		"Authorization": "Bearer " + a.apiKey,
		"Content-Type":  "application/json",
	}

	if strings.Contains(a.model, "codex") {
		var instructions string
		nonSystem := make([]agentTypes.Message, 0, len(messages))
		for _, m := range messages {
			if m.Role == "system" {
				if s, ok := m.Content.(string); ok {
					if instructions != "" {
						instructions += "\n"
					}
					instructions += s
				}
				continue
			}
			nonSystem = append(nonSystem, m)
		}

		body := map[string]any{
			"model":               a.model,
			"input":               copilotResponse.ConvertInput(nonSystem),
			"tools":               copilotResponse.ConvertTools(tools),
			"instructions":        instructions,
			"store":               false,
			"parallel_tool_calls": false,
		}

		result, _, err := go_utils_http.POST[copilotResponse.Output](ctx, a.httpClient, responsesAPI, headers, body, "json")
		if err != nil {
			return nil, fmt.Errorf("http.POST: %w", err)
		}
		if result.Error != nil {
			return nil, fmt.Errorf("http.POST: %s", result.Error.Message)
		}
		out := copilotResponse.ConvertOutput(result)
		return &out, nil
	}

	body := map[string]any{
		"model":    a.model,
		"messages": messages,
		"tools":    tools,
	}
	if provider.SupportTemperature("openai", a.model) {
		body["temperature"] = 0.2
	}
	if provider.SupportReasoningEffort("openai", a.model) {
		body["reasoning_effort"] = provider.GetReasoningLevel()
	}
	result, _, err := go_utils_http.POST[agentTypes.Output](ctx, a.httpClient, chatAPI, headers, body, "json")
	if err != nil {
		return nil, fmt.Errorf("http.POST: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("http.POST: %s", result.Error.Message)
	}

	return &result, nil
}
