package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/skill"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	"github.com/pardnchiu/agenvoy/internal/utils"
)

const (
	baseAPI = "https://generativelanguage.googleapis.com/v1beta/models/"
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
	var systemPrompt string
	var newMessages []Content

	for _, msg := range messages {
		if msg.Role == "system" {
			if content, ok := msg.Content.(string); ok {
				systemPrompt = content
			}
			continue
		}

		message := a.convertToContent(msg)
		newMessages = append(newMessages, message)
	}

	newTools := a.convertToTools(tools)
	apiURL := fmt.Sprintf("%s%s:generateContent?key=%s", baseAPI, a.model, a.apiKey)
	requestBody := a.generateRequestBody(newMessages, systemPrompt, newTools)

	result, _, err := utils.POST[Output](ctx, a.httpClient, apiURL, map[string]string{
		"Content-Type": "application/json",
	}, requestBody, "json")
	if err != nil {
		return nil, fmt.Errorf("utils.POST: %w", err)
	}

	return a.convertToOutput(&result), nil
}

func (a *Agent) convertToContent(message agentTypes.Message) Content {
	content := Content{}
	if message.ToolCallID != "" {
		content.Role = "function"
		data := map[string]any{}
		if contentStr, ok := message.Content.(string); ok {
			data["result"] = contentStr
		}
		content.Parts = []Part{
			{
				FunctionResponse: &FunctionResponse{
					Name:     message.ToolCallID,
					Response: data,
				},
			},
		}
		return content
	}

	role := message.Role
	if role == "assistant" {
		role = "model"
	}
	content.Role = role

	if len(message.ToolCalls) > 0 {
		for _, tool := range message.ToolCalls {
			var args map[string]any
			json.Unmarshal([]byte(tool.Function.Arguments), &args)
			content.Parts = append(content.Parts, Part{
				ThoughtSignature: tool.ThoughtSignature,
				FunctionCall: &FunctionCall{
					Name: tool.Function.Name,
					Args: args,
				},
			})
		}
		return content
	}

	switch v := message.Content.(type) {
	case string:
		content.Parts = []Part{{Text: v}}
	case []agentTypes.ContentPart:
		for _, p := range v {
			switch p.Type {
			case "text":
				content.Parts = append(content.Parts, Part{Text: p.Text})
			case "image_url":
				if p.ImageURL == nil {
					continue
				}
				// * to inlineData
				url := p.ImageURL.URL
				if strings.HasPrefix(url, "data:") {
					if semi := strings.Index(url, ";base64,"); semi != -1 {
						mimeType := url[5:semi]
						b64 := url[semi+8:]
						content.Parts = append(content.Parts, Part{
							InlineData: &InlineData{MimeType: mimeType, Data: b64},
						})
					}
				}
			}
		}
	}

	return content
}

func (a *Agent) convertToTools(tools []toolTypes.Tool) []map[string]any {
	newTools := make([]map[string]any, len(tools))
	for i, tool := range tools {
		var params map[string]any
		json.Unmarshal(tool.Function.Parameters, &params)

		newTools[i] = map[string]any{
			"name":        tool.Function.Name,
			"description": tool.Function.Description,
			"parameters":  params,
		}
	}
	return newTools
}

func (a *Agent) generateRequestBody(messages []Content, prompt string, newTools []map[string]any) map[string]any {
	thinkingConfig := provider.GetThinkingConfig("gemini", a.model)
	level := provider.GetReasoningLevel()

	generationConfig := map[string]any{}
	switch thinkingConfig {
	case "level":
		// Gemini 3.x: thinkingLevel, keep temperature at default (1.0)
		generationConfig["thinkingConfig"] = map[string]any{
			"thinkingLevel": level,
		}
	case "budget":
		// Gemini 2.5: thinkingBudget token count
		generationConfig["temperature"] = 0.2
		generationConfig["thinkingConfig"] = map[string]any{
			"thinkingBudget": provider.ThinkingBudget(level),
		}
	default:
		generationConfig["temperature"] = 0.2
	}
	body := map[string]any{
		"contents":         messages,
		"generationConfig": generationConfig,
	}

	if prompt != "" {
		body["systemInstruction"] = map[string]any{
			"parts": []map[string]any{
				{"text": prompt},
			},
		}
	}

	if len(newTools) > 0 {
		body["tools"] = []map[string]any{
			{"functionDeclarations": newTools},
		}
	}
	return body
}

func (a *Agent) convertToOutput(resp *Output) *agentTypes.Output {
	output := &agentTypes.Output{
		Choices: make([]agentTypes.OutputChoices, 1),
	}

	if resp.UsageMetadata != nil {
		output.Usage = agentTypes.Usage{
			Input:     resp.UsageMetadata.PromptTokenCount - resp.UsageMetadata.CachedContentTokenCount,
			Output:    resp.UsageMetadata.CandidatesTokenCount,
			CacheRead: resp.UsageMetadata.CachedContentTokenCount,
		}
	}

	if len(resp.Candidates) == 0 {
		return output
	}

	candidate := resp.Candidates[0]
	var toolCalls []agentTypes.ToolCall
	var textContent string

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			textContent = part.Text
		} else if part.FunctionCall != nil {
			args := "{}"
			if part.FunctionCall.Args != nil {
				data, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					continue
				}
				args = string(data)
			}

			toolCall := agentTypes.ToolCall{
				ID:               part.FunctionCall.Name,
				Type:             "function",
				ThoughtSignature: part.ThoughtSignature,
			}
			toolCall.Function.Name = part.FunctionCall.Name
			toolCall.Function.Arguments = args
			toolCalls = append(toolCalls, toolCall)
		}
	}

	output.Choices[0].Message = agentTypes.Message{
		Role:      "assistant",
		Content:   textContent,
		ToolCalls: toolCalls,
	}
	output.Choices[0].FinishReason = candidate.FinishReason

	return output
}
