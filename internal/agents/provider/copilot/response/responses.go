package copilotResponse

import (
	"encoding/json"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

// * Responses API response types
type Output struct {
	Output []struct {
		Type    string `json:"type"`
		Role    string `json:"role,omitempty"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content,omitempty"`
		ID        string `json:"id,omitempty"`
		CallID    string `json:"call_id,omitempty"`
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type ToolCall struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

func ConvertInput(messages []agentTypes.Message) []map[string]any {
	result := make([]map[string]any, 0, len(messages))
	for _, m := range messages {
		// * tool result -> function_call_output
		if m.Role == "tool" {
			content := ""
			if s, ok := m.Content.(string); ok {
				content = s
			}
			result = append(result, map[string]any{
				"type":    "function_call_output",
				"call_id": m.ToolCallID,
				"output":  content,
			})
			continue
		}
		// * assistant with tool_calls -> function_call items
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			for _, toolCall := range m.ToolCalls {
				result = append(result, map[string]any{
					"type":      "function_call",
					"call_id":   toolCall.ID,
					"name":      toolCall.Function.Name,
					"arguments": toolCall.Function.Arguments,
				})
			}
			continue
		}
		// * regular message
		result = append(result, map[string]any{
			"role":    m.Role,
			"content": convertContent(m.Role, m.Content),
		})
	}
	return result
}

func convertContent(role string, content any) any {
	textType := "input_text"
	if role == "assistant" {
		textType = "output_text"
	}

	switch v := content.(type) {
	case string:
		return []map[string]any{
			{"type": textType, "text": v},
		}
	case []agentTypes.ContentPart:
		parts := make([]map[string]any, 0, len(v))
		for _, p := range v {
			switch p.Type {
			case "text":
				parts = append(parts, map[string]any{
					"type": textType,
					"text": p.Text,
				})
			case "image_url":
				url := ""
				if p.ImageURL != nil {
					url = p.ImageURL.URL
				}
				parts = append(parts, map[string]any{
					"type":      "input_image",
					"image_url": url,
				})
			default:
				parts = append(parts, map[string]any{
					"type": p.Type,
					"text": p.Text,
				})
			}
		}
		return parts
	default:
		return content
	}
}

func ConvertTools(tools []toolTypes.Tool) []ToolCall {
	result := make([]ToolCall, len(tools))
	for i, t := range tools {
		result[i] = ToolCall{
			Type:        t.Type,
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		}
	}
	return result
}

func ConvertOutput(r Output) agentTypes.Output {
	var msg agentTypes.Message
	msg.Role = "assistant"

	for _, item := range r.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" {
					msg.Content = c.Text
				}
			}
		case "function_call":
			msg.ToolCalls = append(msg.ToolCalls, agentTypes.ToolCall{
				ID:   item.CallID,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      item.Name,
					Arguments: item.Arguments,
				},
			})
		}
	}

	finishReason := "stop"
	if len(msg.ToolCalls) > 0 {
		finishReason = "tool_calls"
	}

	return agentTypes.Output{
		Choices: []agentTypes.OutputChoices{
			{Message: msg, FinishReason: finishReason},
		},
		Usage: agentTypes.Usage{
			Input:  r.Usage.InputTokens,
			Output: r.Usage.OutputTokens,
		},
	}
}
