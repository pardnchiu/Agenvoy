package openaicodex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	copilotResponse "github.com/pardnchiu/agenvoy/internal/agents/provider/copilot/response"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/skill"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const responsesAPI = "https://chatgpt.com/backend-api/codex/responses"

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
	auth, err := a.authHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("a.authHeader: %w", err)
	}

	var instructions string
	var nonSystem []agentTypes.Message
	for _, m := range messages {
		if m.Role == "system" {
			if s, ok := m.Content.(string); ok {
				if instructions != "" {
					instructions += "\n"
				}
				instructions += s
			}
		} else {
			nonSystem = append(nonSystem, m)
		}
	}

	body := map[string]any{
		"model":        a.model,
		"input":        copilotResponse.ConvertInput(nonSystem),
		"tools":        copilotResponse.ConvertTools(tools),
		"instructions": instructions,
		"store":        false,
		"stream":       true,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, responsesAPI, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("http.NewRequestWithContext: %w", err)
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	if a.token != nil && a.token.AccountID != "" {
		req.Header.Set("ChatGPT-Account-Id", a.token.AccountID)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpClient.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, buf.String())
	}

	return parseSSEStream(resp)
}

type sseEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
	Item  *struct {
		Type      string `json:"type"`
		CallID    string `json:"call_id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"item"`
	Response *copilotResponse.Output `json:"response"`
}

func parseSSEStream(resp *http.Response) (*agentTypes.Output, error) {
	var (
		textBuf   strings.Builder
		toolCalls []agentTypes.ToolCall
		usage     agentTypes.Usage
	)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var ev sseEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "response.output_text.delta":
			textBuf.WriteString(ev.Delta)

		case "response.output_item.done":
			if ev.Item != nil && ev.Item.Type == "function_call" {
				toolCalls = append(toolCalls, agentTypes.ToolCall{
					ID:   ev.Item.CallID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      ev.Item.Name,
						Arguments: ev.Item.Arguments,
					},
				})
			}

		case "response.completed":
			if ev.Response != nil {
				usage = agentTypes.Usage{
					Input:     ev.Response.Usage.InputTokens - ev.Response.Usage.InputTokensDetails.CachedTokens,
					Output:    ev.Response.Usage.OutputTokens,
					CacheRead: ev.Response.Usage.InputTokensDetails.CachedTokens,
				}
				if len(toolCalls) == 0 {
					out := copilotResponse.ConvertOutput(*ev.Response)
					if len(out.Choices) > 0 {
						toolCalls = out.Choices[0].Message.ToolCalls
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner: %w", err)
	}

	msg := agentTypes.Message{Role: "assistant"}
	if text := textBuf.String(); text != "" {
		msg.Content = text
	}
	msg.ToolCalls = toolCalls

	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	return &agentTypes.Output{
		Choices: []agentTypes.OutputChoices{
			{Message: msg, FinishReason: finishReason},
		},
		Usage: usage,
	}, nil
}
