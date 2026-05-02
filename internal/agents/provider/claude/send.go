package claude

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
	go_pkg_http "github.com/pardnchiu/go-pkg/http"
)

const (
	messagesAPI = "https://api.anthropic.com/v1/messages"
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
	var systemPrompts []map[string]any
	var newMessages []map[string]any

	for _, msg := range messages {
		if msg.Role == "system" {
			if content, ok := msg.Content.(string); ok && content != "" {
				systemPrompts = append(systemPrompts, map[string]any{
					"type": "text",
					"text": content,
				})
			}
			continue
		}

		message := a.convertToMessage(msg)
		newMessages = append(newMessages, message)
	}

	if len(systemPrompts) > 0 {
		systemPrompts[len(systemPrompts)-1]["cache_control"] = map[string]any{"type": "ephemeral"}
	}

	newTools := a.convertToTools(tools)

	thinkingType := provider.GetThinkingType("claude", a.model)
	level := provider.GetReasoningLevel()

	requestBody := map[string]any{
		"model":      a.model,
		"max_tokens": a.maxOutputTokens(),
		"system":     systemPrompts,
		"messages":   newMessages,
		"tools":      newTools,
	}
	switch thinkingType {
	case "adaptive":
		requestBody["thinking"] = map[string]any{"type": "adaptive"}
		requestBody["output_config"] = map[string]any{"effort": level}
	case "enabled":
		// 4-5: budget_tokens required
		budget := map[string]int{"low": 5000, "high": 32000}[level]
		if budget == 0 {
			budget = 10000
		}
		requestBody["thinking"] = map[string]any{
			"type":          "enabled",
			"budget_tokens": budget,
		}
	default:
		requestBody["temperature"] = 0.2
	}

	result, _, err := go_pkg_http.POST[Output](ctx, a.httpClient, messagesAPI, map[string]string{
		"x-api-key":         a.apiKey,
		"anthropic-version": "2023-06-01",
		"anthropic-beta":    "prompt-caching-2024-07-31",
		"Content-Type":      "application/json",
	}, requestBody, "json")
	if err != nil {
		return nil, fmt.Errorf("http.POST: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("result.Error: %s", result.Error.Message)
	}

	if result.StopReason == "max_tokens" {
		return nil, fmt.Errorf("exceeded max_tokens (%d)", a.maxOutputTokens())
	}

	return a.convertToOutput(&result), nil
}

func (a *Agent) convertToMessage(message agentTypes.Message) map[string]any {
	if message.ToolCallID != "" {
		var toolResultContent any = message.Content
		if parts, ok := message.Content.([]agentTypes.ContentPart); ok {
			var blocks []map[string]any
			for _, p := range parts {
				switch p.Type {
				case "text":
					blocks = append(blocks, map[string]any{"type": "text", "text": p.Text})
				case "image_url":
					if p.ImageURL == nil {
						continue
					}
					mediaType, data, ok := parseDataURL(p.ImageURL.URL)
					if !ok {
						continue
					}
					blocks = append(blocks, map[string]any{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": mediaType,
							"data":       data,
						},
					})
				}
			}
			toolResultContent = blocks
		}
		return map[string]any{
			"role": "user",
			"content": []map[string]any{
				{
					"type":        "tool_result",
					"tool_use_id": message.ToolCallID,
					"content":     toolResultContent,
				},
			},
		}
	}

	if len(message.ToolCalls) > 0 {
		var content []map[string]any
		for _, tool := range message.ToolCalls {
			var input map[string]any
			json.Unmarshal([]byte(tool.Function.Arguments), &input)
			content = append(content, map[string]any{
				"type":  "tool_use",
				"id":    tool.ID,
				"name":  tool.Function.Name,
				"input": input,
			})
		}
		return map[string]any{
			"role":    message.Role,
			"content": content,
		}
	}

	if parts, ok := message.Content.([]agentTypes.ContentPart); ok {
		var content []map[string]any
		for _, part := range parts {
			if part.Type == "text" {
				content = append(content, map[string]any{
					"type": "text",
					"text": part.Text,
				})
			} else if part.Type == "image_url" && part.ImageURL != nil {
				mediaType, data, ok := parseDataURL(part.ImageURL.URL)
				if !ok {
					continue
				}
				content = append(content, map[string]any{
					"type": "image",
					"source": map[string]any{
						"type":       "base64",
						"media_type": mediaType,
						"data":       data,
					},
				})
			}
		}
		return map[string]any{
			"role":    message.Role,
			"content": content,
		}
	}

	return map[string]any{
		"role":    message.Role,
		"content": message.Content,
	}
}

func parseDataURL(url string) (mediaType, data string, ok bool) {
	// data:<mediaType>;base64,<data>
	if len(url) < 5 || url[:5] != "data:" {
		return "", "", false
	}
	rest := url[5:]
	semi := strings.Index(rest, ";base64,")
	if semi < 0 {
		return "", "", false
	}
	return rest[:semi], rest[semi+8:], true
}

func (a *Agent) convertToTools(tools []toolTypes.Tool) []map[string]any {
	newTools := make([]map[string]any, len(tools))
	for i, tool := range tools {
		newTools[i] = map[string]any{
			"name":         tool.Function.Name,
			"description":  tool.Function.Description,
			"input_schema": json.RawMessage(tool.Function.Parameters),
		}
	}
	if len(newTools) > 0 {
		newTools[len(newTools)-1]["cache_control"] = map[string]any{"type": "ephemeral"}
	}
	return newTools
}

func (a *Agent) convertToOutput(resp *Output) *agentTypes.Output {
	output := &agentTypes.Output{
		Choices: make([]agentTypes.OutputChoices, 1),
		Usage: agentTypes.Usage{
			Input:       resp.Usage.InputTokens,
			Output:      resp.Usage.OutputTokens,
			CacheCreate: resp.Usage.CacheCreationInputTokens,
			CacheRead:   resp.Usage.CacheReadInputTokens,
		},
	}

	var toolCalls []agentTypes.ToolCall
	var textContent string

	for _, item := range resp.Content {
		if item.Type == "text" {
			textContent = item.Text
		} else if item.Type == "tool_use" {
			arg := ""
			if item.Input != nil {
				data, err := json.Marshal(item.Input)
				if err != nil {
					continue
				}
				arg = string(data)
			}

			toolCall := agentTypes.ToolCall{
				ID:   item.ID,
				Type: "function",
			}
			toolCall.Function.Name = item.Name
			toolCall.Function.Arguments = arg
			toolCalls = append(toolCalls, toolCall)
		}
	}

	output.Choices[0].Message = agentTypes.Message{
		Role:      "assistant",
		Content:   textContent,
		ToolCalls: toolCalls,
	}

	return output
}
