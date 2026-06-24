package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/agents"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	sessionHistory "github.com/pardnchiu/agenvoy/internal/session/history"
	historyStore "github.com/pardnchiu/agenvoy/internal/session/history/store"
	sessionLog "github.com/pardnchiu/agenvoy/internal/session/log"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

type compactExchange struct {
	start int
	end   int
}

func CompactHistory(ctx context.Context, sessionID string) (int, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("session id is required")
	}

	old, _ := sessionHistory.Get(sessionID)
	if len(old) < 4 {
		syncActionLog(sessionID, old)
		return 0, nil
	}

	agent := SelectAgent(ctx, summaryRouter(), agents.Registry(), "[compact] prune history", false, sessionID)
	if agent == nil {
		return 0, fmt.Errorf("no agent available for compact")
	}

	exchanges := groupExchanges(old)
	if len(exchanges) < 2 {
		syncActionLog(sessionID, old)
		return 0, nil
	}

	removeSet := preFilter(old, exchanges)

	var remaining []int
	for i := range exchanges {
		if !removeSet[i] {
			remaining = append(remaining, i)
		}
	}

	if len(remaining) >= 2 {
		llmRemove, err := identifyRemovable(ctx, agent, old, exchanges, remaining)
		if err != nil {
			slog.Warn("compact: LLM pass failed, using pre-filter only",
				slog.String("session", sessionID),
				slog.String("error", err.Error()))
		} else {
			for idx := range llmRemove {
				removeSet[idx] = true
			}
		}
	}

	if len(removeSet) == 0 {
		syncActionLog(sessionID, old)
		return 0, nil
	}

	kept := make([]agentTypes.Message, 0, len(old))
	removed := 0
	for i, ex := range exchanges {
		if removeSet[i] {
			removed += ex.end - ex.start
			continue
		}
		kept = append(kept, old[ex.start:ex.end]...)
	}
	if removed == 0 {
		syncActionLog(sessionID, old)
		return 0, nil
	}

	if err := sessionHistory.Replace(sessionID, kept); err != nil {
		return 0, fmt.Errorf("history replace: %w", err)
	}

	syncActionLog(sessionID, kept)
	return removed, nil
}

func syncActionLog(sessionID string, messages []agentTypes.Message) {
	var keptContents []string
	for _, msg := range messages {
		if msg.Role == "user" {
			keptContents = append(keptContents, historyStore.ExtractContent(msg.Content))
		}
	}
	sessionLog.RetainExchanges(sessionID, keptContents)
}

func groupExchanges(messages []agentTypes.Message) []compactExchange {
	var list []compactExchange
	current := -1
	for i, msg := range messages {
		if msg.Role == "user" {
			if current >= 0 {
				list = append(list, compactExchange{start: current, end: i})
			}
			current = i
		}
	}
	if current >= 0 {
		list = append(list, compactExchange{start: current, end: len(messages)})
	}
	return list
}

func preFilter(messages []agentTypes.Message, exchanges []compactExchange) map[int]bool {
	removeSet := make(map[int]bool)

	seen := make(map[string]int)
	for i := len(exchanges) - 1; i >= 0; i-- {
		if removeSet[i] {
			continue
		}
		userContent := extractUserContent(messages[exchanges[i].start])
		normalized := strings.TrimSpace(userContent)
		if normalized == "" {
			removeSet[i] = true
			continue
		}
		if _, exists := seen[normalized]; exists {
			removeSet[i] = true
		} else {
			seen[normalized] = i
		}
	}

	return removeSet
}

var metadataBlockRegex = regexp.MustCompile(`(?s)^---\n.*?\n---\n?`)

func extractUserContent(msg agentTypes.Message) string {
	content := historyStore.ExtractContent(msg.Content)
	content = metadataBlockRegex.ReplaceAllString(content, "")
	lines := strings.SplitN(content, "\n", 20)
	start := 0
	for start < len(lines) {
		line := strings.TrimSpace(lines[start])
		if line == "" || strings.HasPrefix(line, "當前時間:") || strings.HasPrefix(line, "工作目錄:") || strings.HasPrefix(line, "傳送者:") || strings.HasPrefix(line, "當前 ") {
			start++
			continue
		}
		break
	}
	if start >= len(lines) {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines[start:], "\n"))
}

func identifyRemovable(ctx context.Context, agent agentTypes.Agent, messages []agentTypes.Message, exchanges []compactExchange, indices []int) (map[int]bool, error) {
	var sb strings.Builder
	for _, i := range indices {
		ex := exchanges[i]
		fmt.Fprintf(&sb, "=== Exchange %d ===\n", i)
		for _, msg := range messages[ex.start:ex.end] {
			content := historyStore.ExtractContent(msg.Content)
			if content == "" && len(msg.ToolCalls) > 0 {
				var names []string
				for _, tc := range msg.ToolCalls {
					names = append(names, tc.Function.Name)
				}
				fmt.Fprintf(&sb, "[%s] <tool_calls: %s>\n", msg.Role, strings.Join(names, ", "))
				continue
			}
			if msg.ToolCallID != "" {
				fmt.Fprintf(&sb, "[tool] %s\n", go_pkg_utils.TruncateString(content, 200))
				continue
			}
			fmt.Fprintf(&sb, "[%s] %s\n", msg.Role, go_pkg_utils.TruncateString(content, 500))
		}
		sb.WriteString("\n")
	}

	prompt := strings.TrimSpace(configs.CompactHistoryPrompt)
	resp, err := agent.Send(ctx, []agentTypes.Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: sb.String()},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("agent send: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	str, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return nil, fmt.Errorf("non-string response")
	}

	return parseRemoveIndices(str, len(exchanges))
}

var compactJSONRegex = regexp.MustCompile(`\{[^{}]*"remove"\s*:\s*\[[^]]*\][^{}]*\}`)

func parseRemoveIndices(raw string, exchangeCount int) (map[int]bool, error) {
	match := compactJSONRegex.FindString(raw)
	if match == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var result struct {
		Remove []int `json:"remove"`
	}
	if err := json.Unmarshal([]byte(match), &result); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}

	dic := make(map[int]bool, len(result.Remove))
	for _, idx := range result.Remove {
		if idx < 0 || idx >= exchangeCount {
			slog.Warn("compact: out-of-range index ignored", slog.Int("index", idx))
			continue
		}
		dic[idx] = true
	}
	return dic, nil
}
