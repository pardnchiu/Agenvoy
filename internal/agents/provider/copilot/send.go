package copilot

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
	"github.com/pardnchiu/agenvoy/internal/utils"
)

const (
	chatAPI      = "https://api.githubcopilot.com/chat/completions"
	responsesAPI = "https://api.githubcopilot.com/responses"
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
	truncated := make([]agentTypes.Message, len(messages))
	copy(truncated, messages)
	for i := range truncated {
		if s, ok := truncated[i].Content.(string); ok {
			truncated[i].Content = utils.TruncateUTF8(s, provider.InputBytes("copilot", a.model))
		}
	}

	if err := a.checkExpires(ctx); err != nil {
		return nil, fmt.Errorf("a.checkExpires: %w", err)
	}

	headers := map[string]string{
		"Authorization":  "Bearer " + a.Refresh.Token,
		"Editor-Version": "vscode/1.95.0",
	}

	if strings.HasPrefix(a.model, "gpt-5.4") || strings.HasSuffix(a.model, "-codex") {
		result, _, err := utils.POST[copilotResponse.Output](ctx, a.httpClient, responsesAPI, headers, map[string]any{
			"model": a.model,
			"input": copilotResponse.ConvertInput(truncated),
			"tools": copilotResponse.ConvertTools(tools),
		}, "json")
		if err != nil {
			return nil, fmt.Errorf("utils.POST: %w", err)
		}
		if result.Error != nil {
			return nil, fmt.Errorf("utils.POST: %s", result.Error.Message)
		}
		out := copilotResponse.ConvertOutput(result)
		return &out, nil
	}

	result, _, err := utils.POST[agentTypes.Output](ctx, a.httpClient, chatAPI, headers, map[string]any{
		"model":       a.model,
		"messages":    truncated,
		"temperature": 0.2,
		"tools":       tools,
	}, "json")
	if err != nil {
		return nil, fmt.Errorf("utils.POST: %w", err)
	}
	if result.Error != nil {
		return nil, fmt.Errorf("utils.POST: %s", result.Error.Message)
	}

	return &result, nil
}
