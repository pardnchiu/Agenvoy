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
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func toolCall(ctx context.Context, exec *toolTypes.Executor, choice agentTypes.OutputChoices, sessionData *agentTypes.AgentSession, events chan<- agentTypes.Event, allowAll bool, alreadyCall map[string]string) (*agentTypes.AgentSession, map[string]string, error) {
	sessionData.ToolHistories = append(sessionData.ToolHistories, choice.Message)

	hasExternalAgent := false
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

		if !allowAll {
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
	}
	return sessionData, alreadyCall, nil
}
