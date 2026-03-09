package agentTypes

import "encoding/json"

type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
	ThoughtSignature string `json:"thought_signature,omitempty"` // Gemini thinking models
}

type Output struct {
	Choices []OutputChoices `json:"choices"`
	Error   *struct {
		Message string      `json:"message"`
		Type    string      `json:"type"`
		Code    json.Number `json:"code"`
	} `json:"error,omitempty"`
}

type OutputChoices struct {
	Message      Message `json:"message"`
	Delta        Message `json:"delta"`
	FinishReason string  `json:"finish_reason,omitempty"`
}
