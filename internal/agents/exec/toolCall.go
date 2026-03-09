package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/tools"
	"github.com/pardnchiu/agenvoy/internal/tools/file"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

func toolCall(ctx context.Context, exec *toolTypes.Executor, choice agentTypes.OutputChoices, sessionData *agentTypes.AgentSession, events chan<- agentTypes.Event, allowAll bool, alreadyCall map[string]string) (*agentTypes.AgentSession, map[string]string, error) {
	sessionData.Messages = append(sessionData.Messages, choice.Message)

	for _, tool := range choice.Message.ToolCalls {
		toolID := strings.TrimSpace(tool.ID)
		toolArg := strings.TrimSpace(tool.Function.Arguments)
		toolName := strings.TrimSpace(tool.Function.Name)
		if idx := strings.Index(toolName, "<|"); idx != -1 {
			toolName = toolName[:idx]
		}

		hash := fmt.Sprintf("%v|%v", toolName, toolArg)
		if cached, ok := alreadyCall[hash]; ok && cached != "" {
			sessionData.Messages = append(sessionData.Messages, agentTypes.Message{
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
				sessionData.Messages = append(sessionData.Messages, agentTypes.Message{
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

		result, err := tools.Execute(ctx, exec, toolName, json.RawMessage(tool.Function.Arguments))
		if err != nil {
			if hint := file.SearchErrorMemory(toolName, err.Error(), 3); hint != "" {
				result = hint
			} else {
				result = "no data"
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
		sessionData.Messages = append(sessionData.Messages, agentTypes.Message{
			Role:       "tool",
			Content:    content,
			ToolCallID: toolID,
		})
	}
	return sessionData, alreadyCall, nil
}
