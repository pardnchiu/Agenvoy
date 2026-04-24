package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/filesystem/errorMemory"
	"github.com/pardnchiu/agenvoy/internal/filesystem/errorMemory/toolError"
	"github.com/pardnchiu/agenvoy/internal/tools"
	toolRegister "github.com/pardnchiu/agenvoy/internal/tools/register"
	toolTypes "github.com/pardnchiu/agenvoy/internal/tools/types"
)

const (
	slotReady          = 0
	slotCached         = 1
	slotSkipped        = 2
	slotStubActivated  = 3
	slotValidateFailed = 4
)

type toolSlot struct {
	idx  int
	id   string
	name string
	args string
	hash string

	state    int
	preMsg   string
	isImage  bool
	imageURL string

	result  string
	execErr string
}

func toolCall(ctx context.Context, exec *toolTypes.Executor, choice agentTypes.OutputChoices, sessionData *agentTypes.AgentSession, events chan<- agentTypes.Event, allowAll bool, alreadyCall map[string]string, toolFailCount map[string]int) (*agentTypes.AgentSession, map[string]string, error) {
	sessionData.ToolHistories = append(sessionData.ToolHistories, choice.Message)

	calls := choice.Message.ToolCalls
	slots := make([]toolSlot, len(calls))
	activatedInBatch := make(map[string]bool)

	for i, tool := range calls {
		toolID := strings.TrimSpace(tool.ID)
		toolArg := strings.TrimSpace(tool.Function.Arguments)
		toolName := strings.TrimSpace(tool.Function.Name)
		if idx := strings.Index(toolName, "<|"); idx != -1 {
			toolName = toolName[:idx]
		}
		hash := fmt.Sprintf("%v|%v", toolName, toolArg)

		slots[i] = toolSlot{
			idx:   i,
			id:    toolID,
			name:  toolName,
			args:  toolArg,
			hash:  hash,
			state: slotReady,
		}

		if toolName != "read_file" {
			if cached, ok := alreadyCall[hash]; ok && cached != "" {
				cachedContent := strings.TrimSpace(cached)
				if strings.HasPrefix(cached, "data:image/") {
					cachedContent = fmt.Sprintf("[%s] image loaded", toolName)
					slots[i].isImage = true
					slots[i].imageURL = cached
				}
				slots[i].state = slotCached
				slots[i].preMsg = cachedContent
				continue
			}
		}

		if exec.StubTools[toolName] || activatedInBatch[toolName] {
			if exec.StubTools[toolName] {
				activateArgs, _ := json.Marshal(map[string]any{"query": "select:" + toolName})
				_, _ = toolRegister.Dispatch(ctx, exec, "search_tools", activateArgs)
				delete(exec.StubTools, toolName)
			}
			activatedInBatch[toolName] = true
			slots[i].state = slotStubActivated
			slots[i].preMsg = fmt.Sprintf("[%s] tool schema just loaded. Re-invoke %s with the correct arguments — the previous call was made against a stub with empty params.", toolName, toolName)
			continue
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
				slots[i].state = slotSkipped
				slots[i].preMsg = "Skipped by user"
				continue
			}
		}

		if earlyErr := validateToolArgs(exec, toolName, toolArg); earlyErr != "" {
			events <- agentTypes.Event{
				Type:     agentTypes.EventToolCall,
				ToolName: toolName,
				ToolArgs: toolArg,
				ToolID:   toolID,
			}
			events <- agentTypes.Event{
				Type:     agentTypes.EventExecError,
				ToolName: toolName,
				ToolID:   toolID,
				Text:     earlyErr,
			}
			toolFailCount[hash]++
			var content string
			if toolFailCount[hash] >= MaxRetry {
				content = fmt.Sprintf("[ABORT] tool=%s 連續 %d 次以相同參數觸發 validator 錯誤: %s\n請改用其他工具或顯著調整參數，不要使用相同工具 %s。", toolName, toolFailCount[hash], earlyErr, toolName)
			} else {
				content = fmt.Sprintf("tool=%s dropped (incomplete args: %s). Do NOT re-issue the same call; if still needed, pivot to a different tool or provide the missing fields from context in a differently-shaped call.", toolName, earlyErr)
			}
			slots[i].state = slotValidateFailed
			slots[i].preMsg = content
			continue
		}
	}

	var wg sync.WaitGroup
	for i := range slots {
		s := &slots[i]
		if s.state != slotReady {
			continue
		}
		if toolRegister.IsConcurrent(s.name) {
			wg.Add(1)
			go func(s *toolSlot) {
				defer wg.Done()
				runToolExec(ctx, exec, s, events)
			}(s)
			continue
		}
		runToolExec(ctx, exec, s, events)
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return sessionData, alreadyCall, err
	}

	hasExternalAgent := false
	hasReviewResult := false

	for i := range slots {
		s := &slots[i]

		switch s.state {
		case slotCached:
			if s.isImage {
				injectImageToUserInput(sessionData, s.imageURL)
			}
			sessionData.ToolHistories = append(sessionData.ToolHistories, agentTypes.Message{
				Role:       "tool",
				Content:    s.preMsg,
				ToolCallID: s.id,
			})
			continue
		case slotSkipped, slotStubActivated, slotValidateFailed:
			msg := agentTypes.Message{
				Role:       "tool",
				Content:    s.preMsg,
				ToolCallID: s.id,
			}
			sessionData.Tools = append(sessionData.Tools, msg)
			sessionData.ToolHistories = append(sessionData.ToolHistories, msg)
			continue
		}

		result := s.result
		if s.execErr != "" {
			toolError.Save(sessionData.ID, s.name, s.args, s.execErr)
			toolFailCount[s.hash]++
			if toolFailCount[s.hash] >= MaxRetry {
				result = fmt.Sprintf("[ABORT] tool=%s 連續 %d 次失敗: %s\n請改用其他工具或顯著調整參數，不要使用相同工具 %s。", s.name, toolFailCount[s.hash], s.execErr, s.name)
			} else {
				if hint := errorMemory.Search(ctx, s.name, s.execErr, 3); hint != "" {
					result = fmt.Sprintf("[RETRY_REQUIRED] tool=%s failed: %s\nrelated_errors: %s\nFix the arguments and call %s again immediately. Do NOT output this message as your response.", s.name, s.execErr, hint, s.name)
				} else {
					result = fmt.Sprintf("[RETRY_REQUIRED] tool=%s failed: %s\nFix the arguments and call %s again immediately. Do NOT output this message as your response.", s.name, s.execErr, s.name)
				}
				delete(alreadyCall, s.hash)
			}
		} else if result == "" || result == "no data" {
			if hint := errorMemory.Search(ctx, s.name, "no data", 3); hint != "" {
				result = hint
			} else {
				result = "no data"
			}
		}

		if s.name != "read_file" {
			alreadyCall[s.hash] = result
		}

		events <- agentTypes.Event{
			Type:     agentTypes.EventToolResult,
			ToolName: s.name,
			ToolID:   s.id,
			Result:   result,
		}

		toolMsgContent := strings.TrimSpace(fmt.Sprintf("[%s] %s", s.name, result))
		if strings.HasPrefix(result, "data:image/") {
			toolMsgContent = fmt.Sprintf("[%s] image loaded", s.name)
			injectImageToUserInput(sessionData, result)
		}
		toolMsg := agentTypes.Message{
			Role:       "tool",
			Content:    toolMsgContent,
			ToolCallID: s.id,
		}
		sessionData.Tools = append(sessionData.Tools, toolMsg)
		sessionData.ToolHistories = append(sessionData.ToolHistories, toolMsg)

		switch s.name {
		case "cross_review_with_external_agents":
			hasExternalAgent = true
			sessionData.VerifyRounds++
			sessionData.VerifyFeedbacks = append(sessionData.VerifyFeedbacks, result)
		case "review_result":
			hasReviewResult = true
		}
	}

	if hasExternalAgent || hasReviewResult {
		sessionData.OldHistories = nil
		if hasExternalAgent {
			sessionData.ToolHistories = trimMessageContext(sessionData.ToolHistories, sessionData.VerifyRounds, sessionData.VerifyFeedbacks)
		} else {
			sessionData.ToolHistories = trimReviewContext(sessionData.ToolHistories)
		}
	}
	return sessionData, alreadyCall, nil
}

func runToolExec(ctx context.Context, exec *toolTypes.Executor, s *toolSlot, events chan<- agentTypes.Event) {
	events <- agentTypes.Event{
		Type:     agentTypes.EventToolCall,
		ToolName: s.name,
		ToolArgs: s.args,
		ToolID:   s.id,
	}
	events <- agentTypes.Event{
		Type:     agentTypes.EventToolCallStart,
		ToolName: s.name,
		ToolID:   s.id,
	}
	result, err := tools.Execute(ctx, exec, s.name, json.RawMessage(s.args))
	if err != nil {
		events <- agentTypes.Event{
			Type:     agentTypes.EventExecError,
			ToolName: s.name,
			ToolID:   s.id,
			Text:     err.Error(),
		}
		s.execErr = err.Error()
		events <- agentTypes.Event{
			Type:     agentTypes.EventToolCallEnd,
			ToolName: s.name,
			ToolID:   s.id,
		}
		return
	}

	if result != "" {
		events <- agentTypes.Event{
			Type:     agentTypes.EventToolCallText,
			ToolName: s.name,
			ToolID:   s.id,
			Text:     result,
		}
	}
	events <- agentTypes.Event{
		Type:     agentTypes.EventToolCallEnd,
		ToolName: s.name,
		ToolID:   s.id,
	}
	s.result = result
}

func validateToolArgs(exec *toolTypes.Executor, toolName, args string) string {
	if exec == nil {
		return ""
	}
	required := requiredFields(exec, toolName)
	if len(required) == 0 {
		return ""
	}

	args = strings.TrimSpace(args)
	var parsed map[string]any
	if args != "" && args != "null" {
		if err := json.Unmarshal([]byte(args), &parsed); err != nil {
			return fmt.Sprintf("invalid JSON for %s: %s. Re-send arguments as a JSON object with required fields: %s",
				toolName, err.Error(), strings.Join(required, ", "))
		}
	}

	var missing []string
	for _, f := range required {
		v, ok := parsed[f]
		if !ok {
			missing = append(missing, f)
			continue
		}
		if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
			missing = append(missing, f)
		}
	}
	if len(missing) == 0 {
		return ""
	}
	return fmt.Sprintf("missing required field(s) %s for %s. All required fields: %s",
		strings.Join(missing, ", "), toolName, strings.Join(required, ", "))
}

func requiredFields(exec *toolTypes.Executor, toolName string) []string {
	lookup := func(list []toolTypes.Tool) []string {
		for _, t := range list {
			if t.Function.Name != toolName {
				continue
			}
			if len(t.Function.Parameters) == 0 {
				return nil
			}
			var schema struct {
				Required []string `json:"required"`
			}
			if err := json.Unmarshal(t.Function.Parameters, &schema); err != nil {
				return nil
			}
			return schema.Required
		}
		return nil
	}
	if r := lookup(exec.AllTools); len(r) > 0 {
		return r
	}
	return lookup(exec.Tools)
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

const MaxVerifyRounds = 3

func trimMessageContext(toolCall []agentTypes.Message, rounds int, feedbacks []string) []agentTypes.Message {
	var firstVersion, feedback string

	for _, m := range toolCall {
		if m.Role != "assistant" || len(m.ToolCalls) == 0 {
			continue
		}
		for _, tc := range m.ToolCalls {
			if tc.Function.Name != "cross_review_with_external_agents" && tc.Function.Name != "invoke_external_agent" {
				continue
			}

			var params struct {
				Result string `json:"result"`
			}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &params); err == nil && params.Result != "" {
				firstVersion = params.Result
			}

			for _, tm := range toolCall {
				if tm.Role == "tool" && tm.ToolCallID == tc.ID {
					if s, ok := tm.Content.(string); ok {
						s = strings.TrimPrefix(s, "[cross_review_with_external_agents] ")
						s = strings.TrimPrefix(s, "[invoke_external_agent] ")
						feedback = s
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

	if rounds >= MaxVerifyRounds {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("已完成 %d 輪外部驗證仍未全員通過，**停止重試**，禁止再次呼叫 cross_review_with_external_agents。請以當前草稿為基礎直接輸出最終結果，並在文末新增 `## 外部驗證未通過理由` 區塊，依序列出各輪 agent 指出的具體問題。\n\n", rounds))
		for i, fb := range feedbacks {
			sb.WriteString(fmt.Sprintf("### Round %d\n%s\n\n", i+1, fb))
		}
		compact = append(compact, agentTypes.Message{
			Role:    "user",
			Content: sb.String(),
		})
		return compact
	}

	if feedback != "" {
		compact = append(compact, agentTypes.Message{
			Role:    "user",
			Content: fmt.Sprintf("以下是第 %d 輪外部驗證回饋（上限 %d 輪），請針對指出的每個問題，**重新呼叫工具查詢**以修正錯誤或補充缺漏，完成後再輸出最終結果：\n\n%s", rounds, MaxVerifyRounds, feedback),
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
