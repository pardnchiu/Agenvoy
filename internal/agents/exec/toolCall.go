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
			sessionData.ToolHistories = append(sessionData.ToolHistories, agentTypes.Message{
				Role:       "tool",
				Content:    strings.TrimSpace(cached),
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

		result, err := tools.Execute(ctx, exec, toolName, json.RawMessage(tool.Function.Arguments))
		if err != nil {
			hash := file.SaveToolError(sessionData.ID, toolName, tool.Function.Arguments, err.Error())
			if hint := file.SearchErrorMemory(toolName, err.Error(), 3); hint != "" {
				result = hint
			} else {
				if strings.HasPrefix(toolName, "api_") {
					_, _ = file.SaveErrorMemory(sessionData.ID, file.ErrorMemory{
						ToolName: toolName,
						Keywords: []string{toolName},
						Symptom:  err.Error(),
						Action:   "工具呼叫失敗，若有備援工具（例如 api_*_1 ↔ api_*_2）請改用；否則回報無法取得資料",
					})
				}
				events <- agentTypes.Event{
					Type:     agentTypes.EventExecError,
					ToolName: toolName,
					ToolID:   toolID,
					Text:     hash,
				}
				result = fmt.Sprintf("no data: %s", hash)
			}
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

		content := strings.TrimSpace(fmt.Sprintf("[%s] %s", toolName, result))
		alreadyCall[hash] = content

		events <- agentTypes.Event{
			Type:     agentTypes.EventToolResult,
			ToolName: toolName,
			ToolID:   toolID,
			Result:   result,
		}
		sessionData.Tools = append(sessionData.Tools, agentTypes.Message{
			Role:       "tool",
			Content:    content,
			ToolCallID: toolID,
		})
		sessionData.ToolHistories = append(sessionData.ToolHistories, agentTypes.Message{
			Role:       "tool",
			Content:    content,
			ToolCallID: toolID,
		})

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
