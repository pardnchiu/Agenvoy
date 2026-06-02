package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	"github.com/pardnchiu/agenvoy/internal/agents/provider"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
	go_pkg_http "github.com/pardnchiu/go-pkg/http"
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
	messages = rewriteSyntheticActivations(messages)

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
	apiURL := fmt.Sprintf("%s%s:generateContent", baseAPI, a.model)
	requestBody := a.generateRequestBody(newMessages, systemPrompt, newTools)

	result, _, err := go_pkg_http.POST[Output](ctx, a.httpClient, apiURL, map[string]string{
		"Content-Type":   "application/json",
		"x-goog-api-key": a.apiKey,
	}, requestBody, "json")
	if err != nil {
		return nil, fmt.Errorf("http.POST: %w", err)
	}

	return a.convertToOutput(&result), nil
}

// rewriteSyntheticActivations folds the synthetic activate_skill call/response
// pair (injected by exec.assignBindingSkill for slash-bound skills) into a
// single user text message. Gemini 3+ requires a real thoughtSignature on
// every functionCall part; synthetic calls never had one and would 400.
func rewriteSyntheticActivations(messages []agentTypes.Message) []agentTypes.Message {
	out := make([]agentTypes.Message, 0, len(messages))
	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		if msg.Role == "assistant" && len(msg.ToolCalls) == 1 {
			tc := msg.ToolCalls[0]
			if tc.Function.Name == "activate_skill" && tc.ThoughtSignature == "" && i+1 < len(messages) {
				next := messages[i+1]
				if next.Role == "tool" && next.ToolCallID == tc.ID {
					activation, _ := next.Content.(string)
					out = append(out, agentTypes.Message{
						Role:    "user",
						Content: activation,
					})
					i++
					continue
				}
			}
		}
		out = append(out, msg)
	}
	return out
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
		if parts, ok := message.Content.([]agentTypes.ContentPart); ok {
			for _, p := range parts {
				if p.Type == "image_url" && p.ImageURL != nil {
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
		stringifyEnums(params)

		newTools[i] = map[string]any{
			"name":        tool.Function.Name,
			"description": tool.Function.Description,
			"parameters":  params,
		}
	}
	return newTools
}

func stringifyEnums(m map[string]any) {
	if vals, ok := m["enum"]; ok {
		if list, ok := vals.([]any); ok {
			for i, v := range list {
				if _, ok := v.(string); !ok {
					list[i] = fmt.Sprintf("%v", v)
				}
			}
		}
	}
	for _, v := range m {
		switch child := v.(type) {
		case map[string]any:
			stringifyEnums(child)
		case []any:
			for _, item := range child {
				if obj, ok := item.(map[string]any); ok {
					stringifyEnums(obj)
				}
			}
		}
	}
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
				raw, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					continue
				}
				args = string(raw)
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
