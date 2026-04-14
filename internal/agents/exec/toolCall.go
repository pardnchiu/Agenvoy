package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/tools/file"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func toolCall(ctx context.Context, exec *toolTypes.Executor, choice agentTypes.OutputChoices, sessionData *agentTypes.AgentSession, events chan<- agentTypes.Event, allowAll bool, alreadyCall map[string]string) (*agentTypes.AgentSession, map[string]string, error) {
	sessionData.ToolHistories = append(sessionData.ToolHistories, choice.Message)

	hasExternalAgent := false
	hasReviewResult := false
	for i, tool := range choice.Message.ToolCalls {
		toolID := strings.TrimSpace(tool.ID)
		toolArg := strings.TrimSpace(tool.Function.Arguments)
		toolName := strings.TrimSpace(tool.Function.Name)
		if idx := strings.Index(toolName, "<|"); idx != -1 {
			toolName = toolName[:idx]
		}

		hash := fmt.Sprintf("%v|%v", toolName, toolArg)
		if cached, ok := alreadyCall[hash]; ok && cached != "" {
			cachedContent := strings.TrimSpace(cached)
			if strings.HasPrefix(cached, "data:image/") {
				cachedContent = fmt.Sprintf("[%s] image loaded", toolName)
				injectImageToUserInput(sessionData, cached)
			}
			sessionData.ToolHistories = append(sessionData.ToolHistories, agentTypes.Message{
				Role:       "tool",
				Content:    cachedContent,
				ToolCallID: toolID,
			})
			continue
		}

		events <- agentTypes.Event{
			Type:     agentTypes.EventToolCall,
			ToolName: toolName,
			ToolArgs: toolArg,
			ToolID:   toolID,
		}

		if !allowAll && !toolRegister.IsReadOnly(toolName) && !strings.HasPrefix(toolName, "api_") {
			replyCh := make(chan bool, 1)
			events <- agentTypes.Event{
				Type:     agentTypes.EventToolConfirm,
				ToolName: toolName,
				ToolArgs: toolArg,
				ToolID:   toolID,
				ReplyCh:  replyCh,
			}
			proceed := <-replyCh
			if !proceed {
				events <- agentTypes.Event{
					Type:     agentTypes.EventToolSkipped,
					ToolName: toolName,
					ToolID:   toolID,
				}
				sessionData.Tools = append(sessionData.Tools, agentTypes.Message{
					Role:       "tool",
					Content:    "Skipped by user",
					ToolCallID: toolID,
				})
				sessionData.ToolHistories = append(sessionData.ToolHistories, agentTypes.Message{
					Role:       "tool",
					Content:    "Skipped by user",
					ToolCallID: toolID,
				})
				continue
			}
		}

		events <- agentTypes.Event{
			Type:     agentTypes.EventToolCallStart,
			ToolName: toolName,
			ToolID:   toolID,
		}

		if i > 0 && strings.HasPrefix(toolName, "api_") {
			select {
			case <-time.After(300 * time.Millisecond):
			case <-ctx.Done():
				return sessionData, alreadyCall, ctx.Err()
			}
		}

		if earlyErr := validateToolArgs(toolName, toolArg); earlyErr != "" {
			events <- agentTypes.Event{
				Type:     agentTypes.EventExecError,
				ToolName: toolName,
				ToolID:   toolID,
				Text:     earlyErr,
			}
			toolMsg := agentTypes.Message{
				Role:       "tool",
				Content:    fmt.Sprintf("[RETRY_REQUIRED] tool=%s: %s", toolName, earlyErr),
				ToolCallID: toolID,
			}
			sessionData.Tools = append(sessionData.Tools, toolMsg)
			sessionData.ToolHistories = append(sessionData.ToolHistories, toolMsg)
			continue
		}

		result, err := tools.Execute(ctx, exec, toolName, json.RawMessage(tool.Function.Arguments))
		if err != nil {
			file.SaveToolError(sessionData.ID, toolName, tool.Function.Arguments, err.Error())
			events <- agentTypes.Event{
				Type:     agentTypes.EventExecError,
				ToolName: toolName,
				ToolID:   toolID,
				Text:     err.Error(),
			}
			if hint := file.SearchErrorMemory(toolName, err.Error(), 3); hint != "" {
				result = fmt.Sprintf("[RETRY_REQUIRED] tool=%s failed: %s\nrelated_errors: %s\nFix the arguments and call %s again immediately. Do NOT output this message as your response.", toolName, err.Error(), hint, toolName)
			} else {
				result = fmt.Sprintf("[RETRY_REQUIRED] tool=%s failed: %s\nFix the arguments and call %s again immediately. Do NOT output this message as your response.", toolName, err.Error(), toolName)
			}
			delete(alreadyCall, hash)
		} else if result == "" || result == "no data" {
			if hint := file.SearchErrorMemory(toolName, "no data", 3); hint != "" {
				result = hint
			} else {
				result = "no data"
			}
		}

		if result != "" {
			events <- agentTypes.Event{
				Type:     agentTypes.EventToolCallText,
				ToolName: toolName,
				ToolID:   toolID,
				Text:     result,
			}
		}

		events <- agentTypes.Event{
			Type:     agentTypes.EventToolCallEnd,
			ToolName: toolName,
			ToolID:   toolID,
		}

		alreadyCall[hash] = result

		events <- agentTypes.Event{
			Type:     agentTypes.EventToolResult,
			ToolName: toolName,
			ToolID:   toolID,
			Result:   result,
		}

		toolMsgContent := strings.TrimSpace(fmt.Sprintf("[%s] %s", toolName, result))
		if strings.HasPrefix(result, "data:image/") {
			toolMsgContent = fmt.Sprintf("[%s] image loaded", toolName)
			injectImageToUserInput(sessionData, result)
		}
		toolMsg := agentTypes.Message{
			Role:       "tool",
			Content:    toolMsgContent,
			ToolCallID: toolID,
		}
		sessionData.Tools = append(sessionData.Tools, toolMsg)
		sessionData.ToolHistories = append(sessionData.ToolHistories, toolMsg)

		switch toolName {
		case "verify_with_external_agent":
			hasExternalAgent = true
		case "review_result":
			hasReviewResult = true
		}
	}

	if hasExternalAgent || hasReviewResult {
		sessionData.OldHistories = nil
		if hasExternalAgent {
			sessionData.ToolHistories = trimMessageContext(sessionData.ToolHistories)
		} else {
			sessionData.ToolHistories = trimReviewContext(sessionData.ToolHistories)
		}
	}
	return sessionData, alreadyCall, nil
}

func validateToolArgs(toolName, args string) string {
	args = strings.TrimSpace(args)
	isEmpty := args == "" || args == "{}" || args == "null"

	type req struct {
		field   string
		example string
	}
	rules := map[string]req{
		"run_command":   {"command", `{"command": "git diff --cached"}`},
		"read_file":     {"path", `{"path": "/path/to/file"}`},
		"write_file":    {"path", `{"path": "/path/to/file", "content": "..."}`},
		"patch_edit":    {"path", `{"path": "/path/to/file", "old_string": "...", "new_string": "..."}`},
		"fetch_page":    {"url", `{"url": "https://..."}`},
		"download_page": {"url", `{"url": "https://..."}`},
		"search_web":    {"query", `{"query": "search terms"}`},
	}

	r, known := rules[toolName]
	if !known {
		return ""
	}
	if isEmpty || !strings.Contains(args, `"`+r.field+`"`) {
		return fmt.Sprintf("missing required field '%s'. Call %s with: %s", r.field, toolName, r.example)
	}
	return ""
}

func injectImageToUserInput(session *agentTypes.AgentSession, dataURL string) {
	part := agentTypes.ContentPart{
		Type:     "image_url",
		ImageURL: &agentTypes.ImageURL{URL: dataURL, Detail: "auto"},
	}
	switch v := session.UserInput.Content.(type) {
	case []agentTypes.ContentPart:
		session.UserInput.Content = append(v, part)
	case string:
		session.UserInput.Content = []agentTypes.ContentPart{
			{Type: "text", Text: v},
			part,
		}
	}
}

func trimMessageContext(toolCall []agentTypes.Message) []agentTypes.Message {
	var firstVersion, feedback string

	for _, m := range toolCall {
		if m.Role != "assistant" || len(m.ToolCalls) == 0 {
			continue
		}
		for _, tc := range m.ToolCalls {
			if tc.Function.Name != "call_external_agent" {
				continue
			}

			var params struct {
				Result string `json:"result"`
			}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err == nil {
				firstVersion = params.Result
			}

			for _, tm := range toolCall {
				if tm.Role == "tool" && tm.ToolCallID == tc.ID {
					if s, ok := tm.Content.(string); ok {
						feedback = strings.TrimPrefix(s, "[call_external_agent] ")
					}
					break
				}
			}
		}
	}

	compact := make([]agentTypes.Message, 0, 2)
	if firstVersion != "" {
		compact = append(compact, agentTypes.Message{
			Role:    "assistant",
			Content: firstVersion,
		})
	}
	if feedback != "" {
		compact = append(compact, agentTypes.Message{
			Role:    "user",
			Content: "以下是外部驗證回饋，請針對指出的每個問題，**重新呼叫工具查詢**以修正錯誤或補充缺漏，完成後再輸出最終結果：\n\n" + feedback,
		})
	}
	return compact
}

func trimReviewContext(toolCall []agentTypes.Message) []agentTypes.Message {
	var draft, feedback string

	for _, m := range toolCall {
		if m.Role != "assistant" || len(m.ToolCalls) == 0 {
			continue
		}
		for _, tc := range m.ToolCalls {
			if tc.Function.Name != "review_result" {
				continue
			}

			var params struct {
				Result string `json:"result"`
			}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err == nil {
				draft = params.Result
			}

			for _, tm := range toolCall {
				if tm.Role == "tool" && tm.ToolCallID == tc.ID {
					if s, ok := tm.Content.(string); ok {
						feedback = strings.TrimPrefix(s, "[內部審查 · ")
					}
					break
				}
			}
		}
	}

	compact := make([]agentTypes.Message, 0, 2)
	if draft != "" {
		compact = append(compact, agentTypes.Message{
			Role:    "assistant",
			Content: draft,
		})
	}
	if feedback != "" {
		compact = append(compact, agentTypes.Message{
			Role:    "user",
			Content: "以下是內部審查回饋，請針對指出的每個問題修正後輸出最終結果：\n\n" + feedback,
		})
	}
	return compact
}
